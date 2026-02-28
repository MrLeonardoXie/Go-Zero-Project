package main

import (
	"context"
	"flag"

	"leonardo/application/article/mq/internal/config"
	"leonardo/application/article/mq/internal/logic"
	"leonardo/application/article/mq/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
)

var configFile = flag.String("f", "etc/article.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	err := c.ServiceConf.SetUp() //启动 Prometheus and Telemetry
	if err != nil {
		panic(err)
	}

	logx.DisableStat()
	svcCtx := svc.NewServiceContext(c)
	ctx := context.Background()
	serviceGroup := service.NewServiceGroup()
	defer serviceGroup.Stop()

	//将Consumers加入到ServiceGroup
	for _, mq := range logic.Consumers(ctx, svcCtx) {
		serviceGroup.Add(mq)
	}

	serviceGroup.Start()
}
