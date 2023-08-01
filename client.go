package toyrpc

import (
	"encoding/json"
	"log"
	"net"
	"reflect"
	"sync"

	"toyrpc/codec"

	"github.com/pkg/errors"
)

var ErrClosed = errors.New("client is closed")

type Client struct {
	codec.Codec
	netConn  net.Conn
	network  string
	address  string
	settings *Settings
	sending  *sync.Mutex // 保证一个返回能完整发送
	mu       *sync.Mutex // 保护seq和pending
	seq      uint64
	pending  map[uint64]*Call
	closed   bool // 用户关闭了客户端
	shutdown bool // 客户端发生严重错误，被强行关闭
}

func NewClient(address string, opts ...CliOption) *Client {
	cli := &Client{
		network:  DefaultNetwork,
		address:  address,
		settings: &DefaultSettings,
		sending:  new(sync.Mutex),
		mu:       new(sync.Mutex),
		seq:      1,
		pending:  make(map[uint64]*Call),
	}
	for _, opt := range opts {
		opt(cli)
	}
	maker, _ := codec.Get(cli.settings.CodecType)
	conn, err := net.Dial(cli.network, cli.address)
	if err != nil {
		panic(err)
	}
	cli.netConn = conn
	cli.Codec = maker(conn)

	if err := json.NewEncoder(cli.netConn).Encode(cli.settings); err != nil {
		_ = cli.Codec.Close()
		panic(err)
	}
	log.Printf("Client start, target server address: %s\n", cli.address)
	go cli.receive()
	return cli
}

func (cli *Client) Call(serviceName, methodName string, args, reply any) error {
	if reflect.TypeOf(reply).Kind() != reflect.Ptr {
		return errors.New("the reply should be pointer")
	}
	req := &Request{
		H: &codec.Header{
			Service: serviceName,
			Method:  methodName,
			SeqId:   cli.getSeqId(),
		},
		Args:  reflect.ValueOf(args),
		Reply: reflect.ValueOf(reply),
	}
	if err := cli.send(req); err != nil {
		return errors.WithMessage(err, "send request fail")
	}
	call := &Call{
		Request: req,
		Done:    make(chan struct{}, 1),
		Err:     nil,
	}
	if err := cli.registry(call); err != nil {
		return errors.WithMessage(err, "registry fail")
	}
	<-call.Done
	delete(cli.pending, call.Request.H.SeqId)
	return call.Err
}

func (cli *Client) send(req *Request) error {
	cli.sending.Lock()
	defer cli.sending.Unlock()
	if err := cli.Write(req.H, req.Args.Interface()); err != nil {
		return errors.WithMessage(err, "client write fail")
	}
	return nil
}

func (cli *Client) getSeqId() uint64 {
	cli.mu.Lock()
	defer cli.mu.Unlock()
	ret := cli.seq
	cli.seq++
	return ret
}

func (cli *Client) registry(call *Call) error {
	cli.mu.Lock()
	defer cli.mu.Unlock()
	if cli.closed || cli.shutdown {
		return ErrClosed
	}
	cli.pending[call.H.SeqId] = call
	return nil
}

func (cli *Client) Close() error {
	cli.mu.Lock()
	defer cli.mu.Unlock()
	if cli.closed {
		return ErrClosed
	}
	cli.closed = true
	return cli.Codec.Close()
}

func (cli *Client) receive() {
	var err error
	var h codec.Header
	for {
		if err = cli.ReadHeader(&h); err != nil {
			break
		}
		call := cli.pending[h.SeqId]
		switch {
		// 处于某些原因取消了，但是服务端仍旧处理了
		case call == nil:
			// TODO 暂时不知道怎么处理
			break
		// 调用出错 返回body应为空
		case h.Err != "":
			call.Err = errors.New(h.Err)
			call.done()
		default:
			err = cli.ReadBody(call.Reply.Interface())
			if err != nil {
				call.Err = err
			}
			call.done()
		}
	}
	cli.terminate(err)
}

// 由于某些内部原因导致了严重错误，需要强行关闭客户端
func (cli *Client) terminate(err error) {
	cli.sending.Lock()
	defer cli.sending.Unlock()
	cli.mu.Lock()
	defer cli.mu.Unlock()
	cli.shutdown = true
	// 将正在pending的调用填写错误原因，全部停止
	for _, call := range cli.pending {
		call.Err = err
		call.done()
	}
}

type CliOption func(cli *Client)

func WithCliNetwork(network string) CliOption {
	return func(cli *Client) {
		cli.network = network
	}
}

func WithCliCodecType(codecType string) CliOption {
	return func(cli *Client) {
		cli.settings.CodecType = codecType
	}
}
