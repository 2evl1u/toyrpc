package toyrpc

import (
	"context"
	"encoding/json"
	"net"
	"reflect"
	"sync"

	. "github.com/2evl1u/toyrpc/log"

	"github.com/2evl1u/toyrpc/codec"
	"github.com/pkg/errors"
)

var ErrClosed = errors.New("client is closed")

type Call struct {
	*request
	done    chan struct{}
	err     error
	invalid bool // 是否超时（无效）
}

func (c *Call) finished() {
	c.done <- struct{}{}
}

type client struct {
	codec.Codec
	netConn    net.Conn
	network    string
	targetAddr string
	settings   *Settings
	sending    *sync.Mutex      // 保证一个返回能完整发送
	mu         *sync.Mutex      // 保护seq和pending
	seq        uint64           // 每一个调用的唯一标识
	pending    map[uint64]*Call // 请求中的调用
	closed     bool             // 用户关闭了客户端
	shutdown   bool             // 客户端发生严重错误，被强行关闭
}

func newClient(address string, opts ...CliOption) *client {
	cli := &client{
		network:    DefaultNetwork,
		targetAddr: address,
		settings:   &DefaultSettings,
		sending:    new(sync.Mutex),
		mu:         new(sync.Mutex),
		seq:        1,
		pending:    make(map[uint64]*Call),
	}
	for _, opt := range opts {
		opt(cli)
	}
	maker, _ := codec.Get(cli.settings.CodecType)
	conn, err := net.Dial(cli.network, cli.targetAddr)
	if err != nil {
		panic(err)
	}
	cli.netConn = conn
	cli.Codec = maker(conn)
	if err := json.NewEncoder(cli.netConn).Encode(cli.settings); err != nil {
		_ = cli.Codec.Close()
		panic(err)
	}
	CommonLogger.Printf("Client start, connect to: %s\n", cli.targetAddr)
	go cli.receive()
	return cli
}

func (cli *client) call(ctx context.Context, serviceName, methodName string, args, reply any) error {
	if reflect.TypeOf(reply).Kind() != reflect.Ptr {
		return errors.New("the reply should be pointer")
	}
	req := &request{
		h: &codec.Header{
			Service: serviceName,
			Method:  methodName,
			SeqId:   cli.getSeqId(),
		},
		args:  reflect.ValueOf(args),
		reply: reflect.ValueOf(reply),
	}
	if err := cli.send(req); err != nil {
		return errors.WithMessage(err, "send request fail")
	}
	call := &Call{
		request: req,
		done:    make(chan struct{}, 1),
		err:     nil,
	}
	if err := cli.registry(call); err != nil {
		return errors.WithMessage(err, "registry fail")
	}
	select {
	// 超时
	case <-ctx.Done():
		call.invalid = true
		return errors.New("call fail: " + ctx.Err().Error())
	case <-call.done:
		cli.mu.Lock()
		delete(cli.pending, call.request.h.SeqId)
		cli.mu.Unlock()
		return call.err
	}
}

// 发送请求
func (cli *client) send(req *request) error {
	cli.sending.Lock()
	defer cli.sending.Unlock()
	if err := cli.Write(req.h, req.args.Interface()); err != nil {
		return errors.WithMessage(err, "client write fail")
	}
	return nil
}

// 获取唯一标识（需要加锁保证线程安全）
func (cli *client) getSeqId() uint64 {
	cli.mu.Lock()
	defer cli.mu.Unlock()
	ret := cli.seq
	cli.seq++
	return ret
}

// 将调用注册到pending中
func (cli *client) registry(call *Call) error {
	cli.mu.Lock()
	defer cli.mu.Unlock()
	if cli.closed || cli.shutdown {
		return ErrClosed
	}
	cli.pending[call.h.SeqId] = call
	return nil
}

func (cli *client) Close() error {
	cli.mu.Lock()
	defer cli.mu.Unlock()
	if cli.closed {
		return ErrClosed
	}
	cli.closed = true
	return cli.Codec.Close()
}

func (cli *client) receive() {
	var err error
	var h codec.Header
	for {
		if err = cli.ReadHeader(&h); err != nil {
			// 读取header出错，证明该连接存在问题，应终止该连接
			break
		}
		call := cli.pending[h.SeqId]
		switch {
		case call == nil:
			// 不可能情况
			break
		// 调用出错 返回body应为空
		case h.Err != "":
			call.err = errors.New(h.Err)
			call.finished()
		default:
			err = cli.ReadBody(call.reply.Interface())
			if err != nil {
				call.err = err
			}
			call.finished()
			// 读取完毕发现是超时的无效请求，删除
			if call.invalid { // 这里先读取再丢弃是为了保证后面的调用返回能正确读取
				cli.mu.Lock()
				delete(cli.pending, call.request.h.SeqId)
				cli.mu.Unlock()
			}
		}
	}
	cli.terminate(err)
}

// 由于某些内部原因导致了严重错误，需要强行关闭客户端
func (cli *client) terminate(err error) {
	cli.sending.Lock()
	defer cli.sending.Unlock()
	cli.mu.Lock()
	defer cli.mu.Unlock()
	cli.shutdown = true
	// 将正在pending的调用填写错误原因，全部停止
	for _, call := range cli.pending {
		call.err = err
		call.finished()
	}
}

type CliOption func(cli *client)

func WithCliNetwork(network string) CliOption {
	return func(cli *client) {
		cli.network = network
	}
}

func WithCliCodecType(codecType string) CliOption {
	return func(cli *client) {
		cli.settings.CodecType = codecType
	}
}
