package toyrpc

import (
	"context"
	"log"
	"math/rand"
	"sync"
	"time"

	"toyrpc/client"

	"github.com/pkg/errors"
)

type SelectMode int

const (
	RandomSelect SelectMode = iota
	RoundRobinSelect
)

type Client struct {
	d          *discovery
	selectMode SelectMode
}

type discovery struct {
	svcMap         map[string]serviceClients
	mu             *sync.RWMutex
	updateInterval time.Duration
	registry       string
}

type serviceClients struct {
	list []cliInfo
	idx  int
}

type cliInfo struct {
	addr        string
	cli         *client.Client
	lastUpdated time.Time
}

// 根据服务名，选择模式来选取一个可用的客户端实例
func (d *discovery) get(serviceName string, mode SelectMode) (*client.Client, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	svcClients := d.svcMap[serviceName]
	var ci cliInfo
	for {
		n := len(svcClients.list)
		if n == 0 {
			return nil, errors.New("[toyrpc] rpc discovery: no available servers")
		}
		switch mode {
		case RandomSelect:
			svcClients.idx = rand.Intn(n)
			ci = svcClients.list[svcClients.idx]
		case RoundRobinSelect:
			ci = svcClients.list[svcClients.idx%n] // servers could be updated, so mode n to ensure safety
			svcClients.idx = (svcClients.idx + 1) % n
		default:
			return nil, errors.New("[toyrpc] rpc discovery: not supported select mode")
		}
		// 该客户端已经过期
		if ci.lastUpdated.Add(d.updateInterval).Before(time.Now()) {
			// 从注册中心获取可用服务实例
			svcAddrs := d.fetch(serviceName)
			// 该实例地址存在，则更新其有效状态
			isUpdated := false
			for _, addr := range svcAddrs {
				if addr == ci.addr {
					ci.lastUpdated = time.Now()
					isUpdated = true
					break
				}
			}
			// 该实例地址不存在，则删除该实例
			if !isUpdated {
				svcClients.list = append(svcClients.list[:svcClients.idx], svcClients.list[svcClients.idx+1:]...)
				continue
			}
		}
		return ci.cli, nil
	}
}

func (d *discovery) fetch(serviceName string) []string {
	// TODO 像注册中心请求服务实例地址
	return nil
}

type CliOpt func(*Client)

// WithSelectMode 用来设置选择服务实例的模式，默认是随机模式
func WithSelectMode(selectMode SelectMode) CliOpt {
	return func(c *Client) {
		c.selectMode = selectMode
	}
}

// WithUpdateInterval 用来设置客户端存储的服务实例的过期时间
func WithUpdateInterval(interval time.Duration) CliOpt {
	return func(c *Client) {
		c.d.updateInterval = interval
	}
}

func NewClient(registry string, opts ...CliOpt) *Client {
	cli := &Client{
		d: &discovery{
			svcMap:         make(map[string]serviceClients),
			mu:             new(sync.RWMutex),
			updateInterval: 5 * time.Minute,
			registry:       registry,
		},
		selectMode: RandomSelect,
	}
	for _, opt := range opts {
		opt(cli)
	}
	return cli
}

func (cli *Client) Call(ctx context.Context, serviceName, methodName string, args, reply any) error {
	c, err := cli.d.get(serviceName, cli.selectMode)
	if err != nil {
		return err
	}
	return c.Call(ctx, serviceName, methodName, args, reply)
}

func (cli *Client) Close() error {
	cli.d.mu.Lock()
	defer cli.d.mu.Unlock()
	for _, sc := range cli.d.svcMap {
		for _, ci := range sc.list {
			if err := ci.cli.Close(); err != nil {
				log.Println(err)
			}
		}
	}
	return nil
}
