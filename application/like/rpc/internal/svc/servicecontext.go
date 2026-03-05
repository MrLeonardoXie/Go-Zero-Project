package svc

import (
	"leonardo/application/like/rpc/internal/config"
	"leonardo/application/like/rpc/internal/model"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config          config.Config
	KqPusherClient  *kq.Pusher
	BizRedis        *redis.Redis
	LikeRecordModel model.LikeRecordModel
	LikeCountModel  model.LikeCountModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Datasource)
	if len(c.CacheRedis) == 0 {
		panic("CacheRedis is empty")
	}
	rds, err := redis.NewRedis(c.CacheRedis[0].RedisConf)
	if err != nil {
		panic(err)
	}

	return &ServiceContext{
		Config:          c,
		KqPusherClient:  kq.NewPusher(c.KqPusherConf.Brokers, c.KqPusherConf.Topic),
		BizRedis:        rds,
		LikeRecordModel: model.NewLikeRecordModel(conn, c.CacheRedis),
		LikeCountModel:  model.NewLikeCountModel(conn, c.CacheRedis),
	}
}
