package toyrpc

import (
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
	svcMap map[string][]struct {
		addr        string
		cli         *client.Client
		lastUpdated time.Time
	}
	mu  *sync.RWMutex
	idx int
}

// 根据服务名，选择模式来选取一个可用的客户端实例
func (d *discovery) get(serviceName string, mode SelectMode) (*client.Client, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	addrList := d.svcMap[serviceName]
	n := len(addrList)
	if n == 0 {
		return nil, errors.New("[toyrpc] rpc discovery: no available servers")
	}
	switch mode {
	case RandomSelect:
		return addrList[rand.Intn(n)].cli, nil
	case RoundRobinSelect:
		s := addrList[d.idx%n] // servers could be updated, so mode n to ensure safety
		d.idx = (d.idx + 1) % n
		return s.cli, nil
	default:
		return nil, errors.New("[toyrpc] rpc discovery: not supported select mode")
	}
}

type CliOpt func(*Client)

func WithSelectMode(selectMode SelectMode) CliOpt {
	return func(c *Client) {
		c.selectMode = selectMode
	}
}

func NewClient(registry string, opts ...CliOpt) *Client {
	cli := &Client{
		d: &discovery{
			svcMap: make(map[string][]struct {
				addr        string
				cli         *client.Client
				lastUpdated time.Time
			}),
			mu:  new(sync.RWMutex),
			idx: 0,
		},
		selectMode: RandomSelect,
	}
	for _, opt := range opts {
		opt(cli)
	}
	return cli
}

// func (cli *Client) Call(ctx context.Context, serviceName, methodName string, args, reply any) error {
// 	addr, err := cli.d.get(serviceName, cli.selectMode)
// 	if err != nil {
// 		return err
// 	}
//
//
// }
//
// func (cli *Client) Close() error {
// 	cli.cliM.mu.Lock()
// 	defer cli.cliM.mu.Unlock()
// 	for addr, c := range cli.cliM.m {
// 		_ = c.Close()
// 		delete(cli.cliM.m, addr)
// 	}
// 	return nil
// }
