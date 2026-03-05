package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"leonardo/application/like/mq/internal/model"
	"leonardo/application/like/mq/internal/svc"
	"leonardo/application/like/mq/internal/types"
	"leonardo/pkg/deltalike"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var (
	ErrInvalidBizId    = errors.New("invalid bizId")
	ErrInvalidObjId    = errors.New("invalid objId")
	ErrInvalidUserId   = errors.New("invalid userId")
	ErrInvalidLikeType = errors.New("invalid likeType")
)

const (
	stateCacheBaseTTL = 600
	stateCacheJitter  = 30
)

type ThumbupLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewThumbupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ThumbupLogic {
	return &ThumbupLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ThumbupLogic) Consume(key, val string) error {
	//1.将Kafka中的字节流[]byte消息，转化为msg types.ThumbupMsg
	var msg types.ThumbupMsg
	if err := json.Unmarshal([]byte(val), &msg); err != nil {
		l.Errorf("[ThumbupConsume] unmarshal val: %s error: %v", val, err)
		return err
	}

	if err := l.validateMsg(&msg); err != nil {
		l.Errorf("[ThumbupConsume] invalid msg: %+v error: %v", msg, err)
		return nil
	}

	/*2.使用事务+幂等，实现like_record和like_count的写入，事务必须绑定在同一个物理连接上*/
	//TransactCtx(...)：从连接池借出一条连接并开启事务，生成 session
	err := l.svcCtx.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		txConn := sqlx.NewSqlConnFromSession(session) //把这个已有 session 包装成 SqlConn 供 model 使用
		recordModel := model.NewLikeRecordModel(txConn, l.svcCtx.Config.CacheRedis)
		countModel := model.NewLikeCountModel(txConn, l.svcCtx.Config.CacheRedis)

		/*幂等：消费时先查旧记录：
		- 没有旧记录 -> 新增 + 计数变更
		- 有旧记录且 like_type 相同 -> 直接返回（幂等）
		- 有旧记录但 like_type 变化 -> 按“旧-1、新+1”更新计数 */
		record, err := recordModel.FindOneByBizIdObjIdUserId(ctx, msg.BizId, msg.ObjId, msg.UserId)
		switch {
		case err == nil: //有旧记录
			oldType := int32(record.LikeType)
			if oldType == msg.LikeType {
				return nil
			}

			record.LikeType = int64(msg.LikeType)
			if err = recordModel.Update(ctx, record); err != nil {
				return err
			}
			//like_count修改
			deltaLike, deltaDislike := deltalike.CalcSwitchDelta(oldType, msg.LikeType)
			return l.applyCountDelta(ctx, countModel, &msg, deltaLike, deltaDislike)
		case err == model.ErrNotFound: //无旧记录
			if _, err = recordModel.Insert(ctx, &model.LikeRecord{
				BizId:    msg.BizId,
				ObjId:    msg.ObjId,
				UserId:   msg.UserId,
				LikeType: int64(msg.LikeType),
			}); err != nil {
				if deltalike.IsDuplicateEntry(err) { //并发时，两条数据同时插入
					current, getErr := recordModel.FindOneByBizIdObjIdUserId(ctx, msg.BizId, msg.ObjId, msg.UserId) //回查
					if getErr != nil {
						return getErr
					}
					oldType := int32(current.LikeType)
					if oldType == msg.LikeType {
						return nil
					}

					current.LikeType = int64(msg.LikeType)
					if upErr := recordModel.Update(ctx, current); upErr != nil {
						return upErr
					}

					deltaLike, deltaDislike := deltalike.CalcSwitchDelta(oldType, msg.LikeType)
					return l.applyCountDelta(ctx, countModel, &msg, deltaLike, deltaDislike)
				}

				return err
			}

			deltaLike, deltaDislike := deltalike.CalcInsertDelta(msg.LikeType)
			return l.applyCountDelta(ctx, countModel, &msg, deltaLike, deltaDislike)
		default:
			return err
		}
	})
	if err != nil {
		return err
	}

	if cacheErr := l.setStateCache(msg.BizId, msg.ObjId, msg.UserId, msg.LikeType); cacheErr != nil {
		l.Logger.Errorf("[ThumbupConsume] setStateCache biz:%s obj:%d user:%d err:%v", msg.BizId, msg.ObjId, msg.UserId, cacheErr)
	}

	return nil
}

func (l *ThumbupLogic) validateMsg(msg *types.ThumbupMsg) error {
	if strings.TrimSpace(msg.BizId) == "" {
		return ErrInvalidBizId
	}
	if msg.ObjId <= 0 {
		return ErrInvalidObjId
	}
	if msg.UserId <= 0 {
		return ErrInvalidUserId
	}
	if msg.LikeType != deltalike.LikeTypeThumbup && msg.LikeType != deltalike.LikeTypeThumbdown {
		return ErrInvalidLikeType
	}
	return nil
}

/* 更新like_count */
func (l *ThumbupLogic) applyCountDelta(ctx context.Context, countModel model.LikeCountModel,
	msg *types.ThumbupMsg, deltaLike, deltaDislike int64) error {
	if deltaLike == 0 && deltaDislike == 0 {
		return nil
	}

	count, err := countModel.FindOneByBizIdObjId(ctx, msg.BizId, msg.ObjId)
	if err == model.ErrNotFound {
		_, err = countModel.Insert(ctx, &model.LikeCount{
			BizId:      msg.BizId,
			ObjId:      msg.ObjId,
			LikeNum:    deltalike.MaxInt64(deltaLike, 0),
			DislikeNum: deltalike.MaxInt64(deltaDislike, 0),
		})
		//两个消费者1号和2号几乎同时插入同一条 like_record，其中一个会报duplicate,这时不是直接失败，而是走“回查 + 更新/忽略”逻辑
		if err != nil && deltalike.IsDuplicateEntry(err) {
			//回查，2号触发duplicate的插入操作,找到1号插入的新数据，赋予count，在下面的逻辑更新数据
			count, err = countModel.FindOneByBizIdObjId(ctx, msg.BizId, msg.ObjId)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	} else if err != nil {
		return err
	}

	count.LikeNum = deltalike.MaxInt64(count.LikeNum+deltaLike, 0)
	count.DislikeNum = deltalike.MaxInt64(count.DislikeNum+deltaDislike, 0)

	return countModel.Update(ctx, count)
}

func Consumers(ctx context.Context, svcCtx *svc.ServiceContext) []service.Service {
	return []service.Service{
		kq.MustNewQueue(svcCtx.Config.KqConsumerConf, NewThumbupLogic(ctx, svcCtx)),
	}
}

func thumbupStateKey(bizId string, objId, userId int64) string {
	return fmt.Sprintf("biz#thumbup#state#%s#%d#%d", bizId, objId, userId)
}

func (l *ThumbupLogic) setStateCache(bizId string, objId, userId int64, likeType int32) error {
	ttl := stateCacheBaseTTL + int(userId%stateCacheJitter)
	return l.svcCtx.BizRedis.SetexCtx(context.Background(), thumbupStateKey(bizId, objId, userId), strconv.FormatInt(int64(likeType), 10), ttl)
}
