package logic

import (
	"context"
	"encoding/json"

	"leonardo/application/like/rpc/internal/model"
	"leonardo/application/like/rpc/internal/svc"
	"leonardo/application/like/rpc/internal/types"
	"leonardo/application/like/rpc/service"
	"leonardo/pkg/deltalike"

	"github.com/zeromicro/go-zero/core/logx"
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

func (l *ThumbupLogic) Thumbup(in *service.ThumbupRequest) (*service.ThumbupResponse, error) {
	// 1) 查用户态记录（幂等 + 计算 delta 必需）
	record, err := l.svcCtx.LikeRecordModel.FindOneByBizIdObjIdUserId(l.ctx, in.BizId, in.ObjId, in.UserId)
	if err != nil && err != model.ErrNotFound {
		return nil, err
	}

	// 2) 幂等短路：同状态重复请求直接返回当前计数（不投递消息）
	if err == nil && int32(record.LikeType) == in.LikeType {
		return l.queryCountResp(in)
	}

	// 3) 计算 delta（基于旧状态 -> 新状态）
	var likeDelta, dislikeDelta int64
	if err == model.ErrNotFound {
		// 没有记录：插入点赞/点踩
		likeDelta, dislikeDelta = deltalike.CalcInsertDelta(in.LikeType)
	} else {
		// 有记录：只能是 0<->1 的切换
		likeDelta, dislikeDelta = deltalike.CalcSwitchDelta(int32(record.LikeType), in.LikeType)
	}

	// 4) 读取当前计数（用于返回“乐观后的展示值”）
	state, err := l.queryCountResp(in)
	if err != nil {
		logx.Errorf("query Obj %d thumbup count failed, err:%v", in.ObjId, err)
		return nil, err
	}

	// 5) 同步 Push：保证“写意图可靠进入 MQ”，否则不要返回成功
	msg := &types.ThumbupMsg{
		BizId:    in.BizId,
		ObjId:    in.ObjId,
		UserId:   in.UserId,
		LikeType: in.LikeType,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		l.Logger.Errorf("[Thumbup] marshal msg: %v error: %v", msg, err)
		return nil, err
	}
	// 不要 GoSafe fire-and-forget：Push 失败要返回错误，让前端别乐观加数（或提示重试）
	if err := l.svcCtx.KqPusherClient.Push(string(data)); err != nil {
		l.Logger.Errorf("[Thumbup] kq push data: %s error: %v", data, err)
		return nil, err
	}

	// 6) 返回“乐观后的计数”（注意防止 <0）
	return &service.ThumbupResponse{
		BizId:      in.BizId,
		ObjId:      in.ObjId,
		LikeNum:    deltalike.MaxInt64(0, state.LikeNum+likeDelta),
		DislikeNum: deltalike.MaxInt64(0, state.DislikeNum+dislikeDelta),
	}, nil
}

func (l *ThumbupLogic) queryCountResp(in *service.ThumbupRequest) (*service.ThumbupResponse, error) {
	count, err := l.svcCtx.LikeCountModel.FindOneByBizIdObjId(l.ctx, in.BizId, in.ObjId)
	if err != nil {
		if err == model.ErrNotFound { //接口返回默认值，不是缓存穿透
			return &service.ThumbupResponse{
				BizId:      in.BizId,
				ObjId:      in.ObjId,
				LikeNum:    0,
				DislikeNum: 0,
			}, nil
		}

		return nil, err
	}

	return &service.ThumbupResponse{
		BizId:      in.BizId,
		ObjId:      in.ObjId,
		LikeNum:    count.LikeNum,
		DislikeNum: count.DislikeNum,
	}, nil
}
