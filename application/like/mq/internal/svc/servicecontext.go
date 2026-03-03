package svc

import (
	"leonardo/application/like/mq/internal/config"
	"leonardo/application/like/mq/internal/model"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config          config.Config
	Conn            sqlx.SqlConn
	LikeRecordModel model.LikeRecordModel
	LikeCountModel  model.LikeCountModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Datasource)

	return &ServiceContext{
		Config:          c,
		Conn:            conn,
		LikeRecordModel: model.NewLikeRecordModel(conn, c.CacheRedis),
		LikeCountModel:  model.NewLikeCountModel(conn, c.CacheRedis),
	}
}
