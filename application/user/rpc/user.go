package main

import (
	"flag"
	"fmt"

	"github.com/MrLeonardoXie/Go-Zero-Project/application/user/rpc/internal/config"
	"github.com/MrLeonardoXie/Go-Zero-Project/application/user/rpc/internal/server"
	"github.com/MrLeonardoXie/Go-Zero-Project/application/user/rpc/internal/svc"
	userservice "github.com/MrLeonardoXie/Go-Zero-Project/application/user/rpc/service"

	"github.com/zeromicro/go-zero/core/conf"
	coreservice "github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/user.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		userservice.RegisterUserServer(grpcServer, server.NewUserServer(ctx))

		if c.Mode == coreservice.DevMode || c.Mode == coreservice.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
