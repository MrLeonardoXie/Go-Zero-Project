package interceptors

import (
	"context"

	"leonardo/pkg/xcode"

	"google.golang.org/grpc"
)

func ServerErrorInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		resp, err = handler(ctx, req)           //handler是RPC的业务逻辑执行函数如Register函数，err：可能是我们xcode中自定义的业务错误码
		return resp, xcode.FromError(err).Err() //将RPC方法返回的error，转换为gPRC中定义的status中的error
	}
}
