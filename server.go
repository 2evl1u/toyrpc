package toyrpc

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"log"
	"net"
	"reflect"
	"sync"

	"toyrpc/codec"

	"github.com/pkg/errors"
)

type Server struct {
	network    string
	address    string
	serviceMap sync.Map
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
func NewServer(opts ...SvrOption) *Server {
	svr := &Server{
		network: DefaultNetwork,
		address: DefaultAddr,
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
		log.Panic(err)
	}
	log.Printf("[toyrpc] Server successfully start at %s\n", listener.Addr().String())
	// 循环接受客户端连接
	for {
		netConn, err := listener.Accept()
		log.Printf("Connect from %s\n", netConn.RemoteAddr().String())
		// 某个连接失败，就close掉，然后跳过接着等待连接
		if err != nil {
			_ = netConn.Close()
			log.Printf("listener accept fail: %s, remote addr: %s\n", err, netConn.RemoteAddr().String())
			continue
		}
		// 连接正常建立之后，先解码settings，获取标识和消息编码类型
		// 默认使用json编码来解码settings
		var settings = new(Settings)
		if err = json.NewDecoder(netConn).Decode(settings); err != nil {
			_ = netConn.Close()
			log.Printf("Decode connect settings fail: %s\n", err)
			continue
		}
		// 判断是不是toyrpc的连接，不是的话直接关闭，打印错误日志
		if settings.MagicNumber != MagicNumber {
			_ = netConn.Close()
			log.Println("Unknown message type")
			continue
		}
		// 获取编码类型
		maker, err := codec.Get(settings.CodecType)
		if err != nil {
			_ = netConn.Close()
			log.Printf("Unknown encoding type: %s\n", settings.CodecType)
			continue
		}
		// 新建toyrpc连接
		conn := s.newConn(maker(netConn))
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
	log.Printf("[toyrpc] Register service: %s. Methods as followed: %s\n", svc.name, nameSli)
	return nil
}

func (s *Server) newConn(cd codec.Codec) *Connection {
	conn := &Connection{
		Codec:   cd,
		sending: new(sync.Mutex),
		wg:      new(sync.WaitGroup),
		svr:     s,
	}
	return conn
}
