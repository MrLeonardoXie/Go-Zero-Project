package svc

import (
	"leonardo/application/applet/internal/config"
	"leonardo/application/follow/rpc/follow"
	"leonardo/application/like/rpc/like"
	"leonardo/application/user/rpc/user"
	"leonardo/pkg/interceptors"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config    config.Config
	UserRPC   user.User
	LikeRPC   like.Like
	FollowRPC follow.Follow
	BizRedis  *redis.Redis
}

func NewServiceContext(c config.Config) *ServiceContext {
	// 创建面向 User 服务配置的通用 gRPC 客户端实例（带发现/负载均衡/熔断能力），自定义拦截器
	userRPC := zrpc.MustNewClient(c.UserRPC, zrpc.WithUnaryClientInterceptor(interceptors.ClientErrorInterceptor()))
	likeRPC := zrpc.MustNewClient(c.LikeRPC, zrpc.WithUnaryClientInterceptor(interceptors.ClientErrorInterceptor()))
	followRPC := zrpc.MustNewClient(c.FollowRPC, zrpc.WithUnaryClientInterceptor(interceptors.ClientErrorInterceptor()))

	return &ServiceContext{
		Config: c,
		//创建user-rpc业务client
		UserRPC:   user.NewUser(userRPC),
		LikeRPC:   like.NewLike(likeRPC),
		FollowRPC: follow.NewFollow(followRPC),
		BizRedis:  redis.New(c.BizRedis.Host, redis.WithPass(c.BizRedis.Pass)),
	}
}
