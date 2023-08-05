package xclient

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"toyrpc"

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
	svcMap         map[string]*serviceClients
	mu             *sync.RWMutex
	updateInterval time.Duration
	registry       string
	r              *rand.Rand
}

type serviceClients struct {
	list []cliDetail
	idx  int
}

type cliDetail struct {
	addr        string
	cli         *toyrpc.Client
	lastUpdated time.Time
}

// 根据服务名，选择模式来选取一个可用的客户端实例
func (d *discovery) get(serviceName string, mode SelectMode) (*toyrpc.Client, error) {
	svcClients, ok := d.svcMap[serviceName]
	// 第一次调用，discovery还未存在对应服务
	if !ok {
		if err := d.update(serviceName); err != nil {
			log.Println(err)
		}
		svcClients = d.svcMap[serviceName]
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	var ci cliDetail
	for {
		n := len(svcClients.list)
		if n == 0 {
			return nil, errors.New("[toyrpc] rpc discovery: no available servers")
		}
		switch mode {
		case RandomSelect:
			svcClients.idx = d.r.Intn(n)
			ci = svcClients.list[svcClients.idx]
		case RoundRobinSelect:
			ci = svcClients.list[svcClients.idx%n] // servers could be updated, so mode n to ensure safety
			svcClients.idx = (svcClients.idx + 1) % n
		default:
			return nil, errors.New("[toyrpc] rpc discovery: not supported select mode")
		}
		// 该客户端已经过期 将其删除
		if ci.lastUpdated.Add(d.updateInterval).Before(time.Now()) {
			svcClients.list = append(svcClients.list[:svcClients.idx], svcClients.list[svcClients.idx+1:]...)
			continue
		}
		return ci.cli, nil
	}
}

// 从注册中心拉取服务实例地址
func (d *discovery) fetch(serviceName string) ([]string, error) {
	resp, err := http.Get(d.registry + toyrpc.DefaultRegisterPath + "?serviceName=" + serviceName)
	if err != nil {
		log.Println("dicovery fetch service addr fail:", err)
	}
	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("read body fail:", err)
	}
	var res []string
	if err = json.Unmarshal(bs, &res); err != nil {
		log.Println("json unmarshal fail:", err)
	}
	return res, nil
}

// 用于更新存于客户端的服务实例列表
func (d *discovery) update(serviceName string) error {
	// 主动向服务中心发出请求获取服务实例地址列表并创建对应的客户端 应该要通过一个goroutine定时执行
	// 从注册中心获取可用服务实例
	svcAddrs, err := d.fetch(serviceName)
	if err != nil {
		return err
	}
	log.Println("fetch services:", svcAddrs)
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, ok := d.svcMap[serviceName]; !ok {
		// 不存在该服务及对应的客户端
		d.svcMap[serviceName] = new(serviceClients)
		// 全加入到list中
		for _, addr := range svcAddrs {
			d.svcMap[serviceName].list = append(d.svcMap[serviceName].list, cliDetail{
				addr:        addr,
				cli:         toyrpc.NewClient(addr),
				lastUpdated: time.Now(),
			})
		}
	} else {
		for _, addr := range svcAddrs {
			existed := false
			for _, ci := range d.svcMap[serviceName].list {
				// 存在则更新lastUpdated标志
				if ci.addr == addr {
					ci.lastUpdated = time.Now()
					existed = true
				}
			}
			if !existed {
				d.svcMap[serviceName].list = append(d.svcMap[serviceName].list, cliDetail{
					addr:        addr,
					cli:         toyrpc.NewClient(addr),
					lastUpdated: time.Now(),
				})
			}
		}
	}
	log.Println("update discovery services")
	return nil
}

func (d *discovery) autoUpdate() {
	for svcName, _ := range d.svcMap {
		if err := d.update(svcName); err != nil {
			log.Println("update service fail:", err)
		}
	}
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
			svcMap:         make(map[string]*serviceClients),
			mu:             new(sync.RWMutex),
			updateInterval: toyrpc.DefaultServerHeartbeatInterval, // 心跳的间隔比服务器超时间隔稍短
			registry:       registry,
			r:              rand.New(rand.NewSource(time.Now().UnixNano())),
		},
		selectMode: RandomSelect,
	}
	for _, opt := range opts {
		opt(cli)
	}
	// 开启一个goroutine来定时更新discovery
	go func() {
		ticker := time.NewTicker(cli.d.updateInterval)
		for {
			select {
			case <-ticker.C:
				cli.d.autoUpdate()
			}
		}
	}()
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
