package config

import (
	"leonardo/pkg/consul"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	DataSource string
	CacheRedis cache.CacheConf
	// BizRedis   redis.RedisConf user-rpc中暂未使用BizRedis
	Consul consul.Conf
}
