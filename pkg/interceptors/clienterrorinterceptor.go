package interceptors

import (
	"context"

	"leonardo/pkg/xcode"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

func ClientErrorInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		err := invoker(ctx, method, req, reply, cc, opts...)
		if err != nil {
			grpcStatus, _ := status.FromError(err)
			xc := xcode.GrpcStatusToXCode(grpcStatus)
			err = errors.WithMessage(xc, grpcStatus.Message()) // sts:{Code:10001, Message:"注册名字不能为空"}, "10001"
		}

		return err
	}
}
