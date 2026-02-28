package logic

import (
	"context"
	"strconv"
	"time"

	"leonardo/application/article/rpc/internal/code"
	"leonardo/application/article/rpc/internal/model"
	"leonardo/application/article/rpc/internal/svc"
	"leonardo/application/article/rpc/internal/types"
	"leonardo/application/article/rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PublishLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPublishLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublishLogic {
	return &PublishLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PublishLogic) Publish(in *pb.PublishRequest) (*pb.PublishResponse, error) {
	if in.UserId <= 0 {
		return nil, code.UserIdInvalid
	}
	if len(in.Title) == 0 {
		return nil, code.ArticleTitleCantEmpty
	}
	if len(in.Content) == 0 {
		return nil, code.ArticleContentCantEmpty
	}
	ret, err := l.svcCtx.ArticleModel.Insert(l.ctx, &model.Article{
		AuthorId:    in.UserId,
		Title:       in.Title,
		Content:     in.Content,
		Description: in.Description,
		Cover:       in.Cover,
		Status:      types.ArticleStatusVisible, // 正常逻辑不会这样写，这里为了演示方便
		PublishTime: time.Now(),
		CreateTime:  time.Now(),
		UpdateTime:  time.Now(),
	})
	if err != nil {
		l.Logger.Errorf("Publish Insert req: %v error: %v", in, err)
		return nil, err
	}

	articleId, err := ret.LastInsertId()
	if err != nil {
		l.Logger.Errorf("LastInsertId error: %v", err)
		return nil, err
	}

	/* 缓存同步与更新：在用户发布文章成功后，实时地更新 Redis 里的有序集合（ZSET）
	   举例：用户 ID (UserId) 是 123，他刚刚发布了一篇文章，数据库生成的文章 ID (articleId) 是 999。*/
	var (
		articleIdStr   = strconv.FormatInt(articleId, 10)
		publishTimeKey = articlesKey(in.UserId, types.SortPublishTime) //拿到排行榜的最新发布榜ID，打印: "user:123:sort:time"
		likeNumKey     = articlesKey(in.UserId, types.SortLikeCount)   //同上，打印: "user:123:sort:like"
	)

	b, _ := l.svcCtx.BizRedis.ExistsCtx(l.ctx, publishTimeKey)
	//如果Redis中没有该缓存，那么就不添加该数据到Redis
	if b {
		// 执行ZaddCtx(l.ctx, publishTimeKey, 1740218400, "999")
		_, err = l.svcCtx.BizRedis.ZaddCtx(l.ctx, publishTimeKey, time.Now().Unix(), articleIdStr)
		if err != nil {
			logx.Errorf("ZaddCtx req: %v error: %v", in, err)
		}
	}
	b, _ = l.svcCtx.BizRedis.ExistsCtx(l.ctx, likeNumKey)
	if b {
		_, err = l.svcCtx.BizRedis.ZaddCtx(l.ctx, likeNumKey, 0, articleIdStr)
		if err != nil {
			logx.Errorf("ZaddCtx req: %v error: %v", in, err)
		}
	}

	return &pb.PublishResponse{ArticleId: articleId}, nil
}
