package main

import (
	"context"
	"flag"

	"leonardo/application/like/mq/internal/config"
	"leonardo/application/like/mq/internal/logic"
	"leonardo/application/like/mq/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
)

var configFile = flag.String("f", "etc/like.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	svcCtx := svc.NewServiceContext(c)
	ctx := context.Background()
	serviceGroup := service.NewServiceGroup()
	defer serviceGroup.Stop()

	for _, mq := range logic.Consumers(ctx, svcCtx) {
		serviceGroup.Add(mq)
	}

	serviceGroup.Start() //mq 往往有多个消费者/订阅器，需要同时启动多个服务并统一管理生命周期，所以用 servicegroup 来一起启动/优雅退出
}
