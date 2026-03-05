package svc

import (
	"leonardo/application/like/mq/internal/config"
	"leonardo/application/like/mq/internal/model"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config          config.Config
	Conn            sqlx.SqlConn
	BizRedis        *redis.Redis
	LikeRecordModel model.LikeRecordModel
	LikeCountModel  model.LikeCountModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Datasource) //创建连接池，将连接池的入口赋值给conn，后续通过conn来从连接池拿连接
	if len(c.CacheRedis) == 0 {
		panic("CacheRedis is empty")
	}
	rds, err := redis.NewRedis(c.CacheRedis[0].RedisConf)
	if err != nil {
		panic(err)
	}

	return &ServiceContext{
		Config:          c,
		Conn:            conn,
		BizRedis:        rds,
		LikeRecordModel: model.NewLikeRecordModel(conn, c.CacheRedis),
		LikeCountModel:  model.NewLikeCountModel(conn, c.CacheRedis),
	}
}
