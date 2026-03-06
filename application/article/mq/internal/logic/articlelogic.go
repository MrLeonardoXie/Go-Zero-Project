package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"leonardo/application/article/mq/internal/svc"
	"leonardo/application/article/mq/internal/types"
	"leonardo/application/user/rpc/user"

	"github.com/elastic/go-elasticsearch/v8/esutil"
	"github.com/zeromicro/go-zero/core/logx"
)

type ArticleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewArticleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ArticleLogic {
	return &ArticleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ArticleLogic) Consume(_, val string) error {
	logx.Infof("Consume msg val: %s", val)
	var msg *types.CanalArticleMsg
	err := json.Unmarshal([]byte(val), &msg)
	if err != nil {
		logx.Errorf("Consume val: %s error: %v", val, err)
		return err
	}

	return l.articleOperate(msg)
}

/*
	1.补偿，保证最终一致性
	2.写入数据到ES
*/
func (l *ArticleLogic) articleOperate(msg *types.CanalArticleMsg) error {
	if len(msg.Data) == 0 {
		return nil
	}

	var esData []*types.ArticleEsMsg
	for _, d := range msg.Data {
		status, _ := strconv.Atoi(d.Status)
		likNum, _ := strconv.ParseInt(d.LikeNum, 10, 64)
		articleId, _ := strconv.ParseInt(d.ID, 10, 64)
		authorId, _ := strconv.ParseInt(d.AuthorId, 10, 64)

		t, err := time.ParseInLocation("2006-01-02 15:04:05", d.PublishTime, time.Local)
		publishTimeKey := articlesKey(d.AuthorId, 0)
		likeNumKey := articlesKey(d.AuthorId, 1)

		//1.更新Redis，实现“最终一致性”
		switch status {
		case types.ArticleStatusVisible: //文章状态为可见
			b, _ := l.svcCtx.BizRedis.ExistsCtx(l.ctx, publishTimeKey) //判断该缓存是否存在，判断的时候使用了熔断操作
			if b {
				/* ZddCtx操作逻辑：操作ZSET列表
				如果 d.ID 这个 member 不存在：新增到有序集合
				如果已存在：更新该 member 的 score（这里是 t.Unix()）
				*/
				_, err = l.svcCtx.BizRedis.ZaddCtx(l.ctx, publishTimeKey, t.Unix(), d.ID)
				if err != nil {
					l.Logger.Errorf("ZaddCtx key: %s req: %v error: %v", publishTimeKey, d, err)
					return err //更新失败，Kafka中的offset不更新，该消费数据继续重试
				}
			}
			b, _ = l.svcCtx.BizRedis.ExistsCtx(l.ctx, likeNumKey)
			if b {
				_, err = l.svcCtx.BizRedis.ZaddCtx(l.ctx, likeNumKey, likNum, d.ID)
				if err != nil {
					l.Logger.Errorf("ZaddCtx key: %s req: %v error: %v", likeNumKey, d, err)
				}
			}
		case types.ArticleStatusUserDelete: //文件状态为删除
			_, err = l.svcCtx.BizRedis.ZremCtx(l.ctx, publishTimeKey, d.ID)
			if err != nil {
				l.Logger.Errorf("ZremCtx key: %s req: %v error: %v", publishTimeKey, d, err)
			}
			_, err = l.svcCtx.BizRedis.ZremCtx(l.ctx, likeNumKey, d.ID)
			if err != nil {
				l.Logger.Errorf("ZremCtx key: %s req: %v error: %v", likeNumKey, d, err)
			}
		}

		//使用UserRPC找到User的信息
		u, err := l.svcCtx.UserRPC.FindById(l.ctx, &user.FindByIdRequest{
			UserId: authorId,
		})
		if err != nil {
			l.Logger.Errorf("FindById userId: %d error: %v", authorId, err)
			return err
		}

		//聚合User数据以及Article数据，构建esData
		esData = append(esData, &types.ArticleEsMsg{ //esData是一个slice
			ArticleId:   articleId,
			AuthorId:    authorId,
			AuthorName:  u.Username,
			Title:       d.Title,
			Content:     d.Content,
			Description: d.Description,
			Status:      status,
			LikeNum:     likNum,
		})
	}
	//2.写入ES数据库
	err := l.BatchUpSertToEs(l.ctx, esData)
	if err != nil {
		l.Logger.Errorf("BatchUpSertToEs data: %v error: %v", esData, err)
	}

	return err
}

// UpSert：如果这条数据不存在，那么就INSERT; 如果这条数据存在，那么就UPDATE
func (l *ArticleLogic) BatchUpSertToEs(ctx context.Context, data []*types.ArticleEsMsg) error {
	if len(data) == 0 {
		return nil
	}

	bi, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
		Client: l.svcCtx.Es.Client,
		Index:  "article-index",
	})
	if err != nil {
		return err
	}

	for _, d := range data {
		v, err := json.Marshal(d) //eg:{article_id:10003,author_id:2,author_name:leo,title:Kafka实战,content:Canal + Kafka + ES,description:链路说明,status:2,like_num:9}
		if err != nil {
			return err
		}

		payload := fmt.Sprintf(`{"doc":%s,"doc_as_upsert":true}`, string(v))
		err = bi.Add(ctx, esutil.BulkIndexerItem{
			Action:     "update",
			DocumentID: fmt.Sprintf("%d", d.ArticleId),
			Body:       strings.NewReader(payload),
			OnSuccess: func(ctx context.Context, item esutil.BulkIndexerItem, item2 esutil.BulkIndexerResponseItem) {
			},
			OnFailure: func(ctx context.Context, item esutil.BulkIndexerItem, item2 esutil.BulkIndexerResponseItem, err error) {
			},
		})
		if err != nil {
			return err
		}
	}

	return bi.Close(ctx) //调用：close(bi.queue)，使用close就会把缓冲队列里的 item flush 出去，即发 HTTP bulk 请求到 ES
}

func articlesKey(uid string, sortType int32) string {
	return fmt.Sprintf("biz#articles#%s#%d", uid, sortType)
}
