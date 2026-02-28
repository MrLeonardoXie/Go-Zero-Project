package consul

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/netx"
	"github.com/zeromicro/go-zero/core/proc"
)

const (
	allEths  = "0.0.0.0"
	envPodIP = "POD_IP"

	defaultTTL    = 10
	defaultTicker = time.Second
)

type Conf struct {
	Host string
	Key  string
	Tags []string          `json:",optional"`
	Meta map[string]string `json:",optional"`
	TTL  int               `json:"ttl,optional"`
}

func Register(conf Conf, listenOn string) error {
	lo := figureOutListenOn(listenOn) //效验地址
	host, pt, err := net.SplitHostPort(lo)
	if err != nil {
		return err
	}
	port, err := strconv.Atoi(pt)
	if err != nil {
		return err
	}
	cli, err := api.NewClient(&api.Config{
		Scheme:  "http",
		Address: conf.Host,
	})
	if err != nil {
		return err
	}
	if conf.TTL <= 0 {
		conf.TTL = defaultTTL
	}

	ttl := fmt.Sprintf("%ds", conf.TTL)
	expiredTTL := fmt.Sprintf("%ds", conf.TTL*3)
	id := genID(conf.Key, host, port)
	asg := &api.AgentServiceRegistration{
		ID:      id,
		Name:    conf.Key,
		Tags:    conf.Tags,
		Meta:    conf.Meta,
		Port:    port,
		Address: host,
		Checks: []*api.AgentServiceCheck{
			{
				CheckID:                        id,  // 服务节点的名称
				TTL:                            ttl, // 健康检查间隔
				Status:                         "passing",
				DeregisterCriticalServiceAfter: expiredTTL, // 注销时间，相当于过期时间
			},
		},
	}
	err = cli.Agent().ServiceRegister(asg)
	if err != nil {
		return err
	}

	check := api.AgentServiceCheck{TTL: ttl, Status: "passing", DeregisterCriticalServiceAfter: expiredTTL}
	err = cli.Agent().CheckRegister(&api.AgentCheckRegistration{ID: id, Name: conf.Key, ServiceID: id, AgentServiceCheck: check})
	if err != nil {
		return err
	}

	ttlTicker := time.Duration(conf.TTL-1) * time.Second //默认把定时周期设成 TTL - 1 秒（比过期时间早 1 秒触发）
	if ttlTicker < time.Second {                         //如果算出来太小（小于 1 秒，可能 TTL 配得太小或异常），就用兜底值 defaultTicker，防止频繁触发或无效配置
		ttlTicker = defaultTicker
	}

	//按 ttlTicker 周期调用 UpdateTTL，不断把服务状态标记为 passing，防止 TTL 检查过期后被 Consul 判死。
	go func() {
		ticker := time.NewTicker(ttlTicker)
		defer ticker.Stop()

		for range ticker.C {
			err = cli.Agent().UpdateTTL(id, "", "passing")
			if err != nil {
				logx.Errorf("UpdateTTL id: %s error: %v", id, err)
			}
		}
	}()

	//进程退出时执行 ServiceDeregister(asg.ID)，把服务从 Consul 注销，避免注册中心里残留脏实例
	proc.AddShutdownListener(func() {
		err = cli.Agent().ServiceDeregister(asg.ID)
		if err != nil {
			logx.Errorf("ServiceDeregister id: %s error: %v", asg.ID, err)
		}
		logx.Infof("ServiceDeregister id: %s success", asg.ID)
	})

	return nil
}

func genID(key, host string, port int) string {
	return fmt.Sprintf("%s-%s-%d", key, host, port)
}

func figureOutListenOn(listenOn string) string {
	fields := strings.Split(listenOn, ":")
	if len(fields) == 0 {
		return listenOn
	}
	host := fields[0]
	if len(host) > 0 && host != allEths {
		return listenOn
	}
	ip := os.Getenv(envPodIP) // 确定服务注册/对外上报用的 IP，逻辑是“优先环境变量，没有就自动探测内网 IP”
	if len(ip) == 0 {
		ip = netx.InternalIp()
	}
	if len(ip) == 0 {
		return listenOn
	}

	return strings.Join(append([]string{ip}, fields[1:]...), ":")
	/*
			ip = "10.0.0.8"，fields = ["127.0.0.1","8080"]
		  	结果：10.0.0.8:8080
	*/
}
