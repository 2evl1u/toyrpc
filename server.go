package toyrpc

import (
	"encoding/json"
	"log"
	"net"

	"toyrpc/codec"
)

type Server struct {
	network string
	address string
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
		conn := NewConn(maker(netConn))
		go conn.Handle()
	}
}
