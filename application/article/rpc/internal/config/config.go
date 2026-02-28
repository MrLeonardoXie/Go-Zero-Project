package config

import (
	"leonardo/pkg/consul"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	DataSource string
	CacheRedis cache.CacheConf // CacheRedis的配置参数, 用于创建CacheRedis
	BizRedis   redis.RedisConf // BizRedis的配置参数, 用于创建BizRedis
	Consul     consul.Conf
}
