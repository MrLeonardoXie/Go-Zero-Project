package svc

import (
	"leonardo/application/article/rpc/internal/config"
	"leonardo/application/article/rpc/internal/model"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"golang.org/x/sync/singleflight"
)

type ServiceContext struct {
	Config            config.Config
	ArticleModel      model.ArticleModel
	BizRedis          *redis.Redis
	SingleFlightGroup singleflight.Group
}

func NewServiceContext(c config.Config) *ServiceContext {
	rds, err := redis.NewRedis(redis.RedisConf{
		Host: c.BizRedis.Host,
		Pass: c.BizRedis.Pass,
		Type: c.BizRedis.Type,
	})
	if err != nil {
		panic(err)
	}

	return &ServiceContext{
		Config: c,
		//CacheRedis: 主要是给 model 层做 MySQL 结果缓存 用的，生成行记录缓存
		ArticleModel: model.NewArticleModel(sqlx.NewMysql(c.DataSource), c.CacheRedis),
		BizRedis:     rds, //BizRedis: 用于业务层面的数据缓存，如缓存VerificationCode
	}
}
