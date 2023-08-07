package toyrpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"net"
	"net/http"
	"reflect"
	"sync"
	"time"

	. "toyrpc/log"

	"toyrpc/codec"

	"github.com/pkg/errors"
)

type Server struct {
	network           string
	address           string
	registry          string
	serviceMap        sync.Map
	heartbeatInterval time.Duration
}

type service struct {
	name string
	self reflect.Value
	mm   map[string]*reflect.Method
	svr  *Server
}

type SvrOption func(server *Server)

func WithSvrNetwork(network string) SvrOption {
	return func(s *Server) {
		s.network = network
	}
}

func WithSvrAddress(address string) SvrOption {
	return func(s *Server) {
		s.address = address
	}
}

// NewServer 如果不指定网络类型，默认tcp；如果不指定端口，则默认7788端口
func NewServer(registry string, opts ...SvrOption) *Server {
	svr := &Server{
		network:           DefaultNetwork,
		address:           DefaultAddr,
		registry:          registry,
		heartbeatInterval: DefaultServerHeartbeatInterval,
	}
	for _, opt := range opts {
		opt(svr)
	}
	return svr
}

func (s *Server) Start() {
	listener, err := net.Listen(s.network, s.address)
	// 启动监听失败，直接panic
	if err != nil {
		ErrorLogger.Panic(err)
	}
	CommonLogger.Printf("Server successfully start at %s\n", listener.Addr().String())
	// 循环接受客户端连接
	for {
		netConn, err := listener.Accept()
		CommonLogger.Printf("Connect from %s\n", netConn.RemoteAddr().String())
		// 某个连接失败，就close掉，然后跳过接着等待连接
		if err != nil {
			_ = netConn.Close()
			ErrorLogger.Printf("Listener accept fail: %s, remote port: %s\n", err, netConn.RemoteAddr().String())
			continue
		}
		// 连接正常建立之后，先解码settings，获取标识和消息编码类型
		// 默认使用json编码来解码settings
		var settings = new(Settings)
		if err = json.NewDecoder(netConn).Decode(settings); err != nil {
			_ = netConn.Close()
			ErrorLogger.Printf("Decode connect settings fail: %s\n", err)
			continue
		}
		// 判断是不是toyrpc的连接，不是的话直接关闭，打印错误日志
		if settings.MagicNumber != MagicNumber {
			_ = netConn.Close()
			ErrorLogger.Println("Unknown message type")
			continue
		}
		// 获取编码类型
		maker, err := codec.Get(settings.CodecType)
		if err != nil {
			_ = netConn.Close()
			ErrorLogger.Printf("Unknown encoding type: %s\n", settings.CodecType)
			continue
		}
		// 新建toyrpc连接
		conn := &Connection{
			Codec:   maker(netConn),
			sending: new(sync.Mutex),
			wg:      new(sync.WaitGroup),
			svr:     s,
		}
		go conn.Handle()
	}
}

// AsService 将一个结构体作为一个服务，会注册特定方法签名的方法
// 一个方法想要被注册为rpc方法，需要满足以下几个条件:
// 1. 方法所属类型是导出的
// 2. 方法本身是导出的
// 3. 两个入参，均为导出或内置类型，且第二个入参需为指针类型
// 4. 返回值是error接口类型
func (s *Server) AsService(target any) error {
	// 1 创建服务
	svc := &service{
		name: reflect.Indirect(reflect.ValueOf(target)).Type().Name(),
		self: reflect.ValueOf(target),
		svr:  s,
	}
	if !ast.IsExported(svc.name) {
		return errors.New(fmt.Sprintf("%s is not exported", svc.name))
	}
	// 2 通过反射寻找符合条件的方法签名
	svc.mm = make(map[string]*reflect.Method)
	targetType := reflect.TypeOf(target)
	for i := 0; i < targetType.NumMethod(); i++ {
		method := targetType.Method(i)
		// 入参必须为3个，第1个为receiver，第2个为args，第3个为reply
		if method.Type.NumIn() != 3 || method.Type.NumOut() != 1 {
			continue
		}
		// 返回值不是error接口类型
		if method.Type.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
			continue
		}
		// 入参必须是导出或者内置类型
		argType, replyType := method.Type.In(1), method.Type.In(2)
		if !(ast.IsExported(argType.Name()) || argType.PkgPath() == "") ||
			!(ast.IsExported(replyType.Name()) || replyType.PkgPath() == "") {
			continue
		}
		// 注册方法
		svc.mm[method.Name] = &method
	}
	// 3 注册服务
	if _, existed := s.serviceMap.LoadOrStore(svc.name, svc); existed {
		return errors.New("[toyrpc] Service already existed: " + svc.name)
	}
	nameSli := make([]string, 0)
	for name := range svc.mm {
		nameSli = append(nameSli, name)
	}
	CommonLogger.Printf("Register service: %s. Methods as followed: %s\n", svc.name, nameSli)
	// 向注册中心发送心跳
	svc.heartbeat()
	go func() {
		// 心跳的间隔比服务器超时间隔稍短
		ticker := time.NewTicker(s.heartbeatInterval)
		for {
			select {
			case <-ticker.C:
				svc.heartbeat()
				CommonLogger.Println("Send heartbeat to registry")
			}
		}
	}()
	return nil
}

// 发送心跳，指示注册中心该服务存活
func (s *service) heartbeat() {
	body := svcUpdateMapping{
		ServiceName: s.name,
		ServiceAddr: s.svr.address,
	}
	bs, _ := json.Marshal(body)
	buffer := bytes.NewBuffer(bs)
	resp, err := http.Post(s.svr.registry+DefaultRegisterPath, "application/json", buffer)
	if err != nil {
		ErrorLogger.Printf("Send heartbeat post request fail: %s\n", err)
	}
	if resp.StatusCode != http.StatusOK {
		ErrorLogger.Printf("Heartbeat response err, status code: %s\n", resp.StatusCode)
	}
}
