package toyrpc

import (
	"encoding/json"
	"log"
	"net"

	"toyrpc/codec"
)

const MagicNumber = 0x3bef5c

type Settings struct {
	MagicNumber int
	CodecType   string
}

var DefaultSettings = &Settings{
	MagicNumber: MagicNumber,
	CodecType:   codec.GobType,
}

type Server struct {
	network string
	address string
}

type Option func(server *Server)

func WithNetwork(network string) Option {
	return func(s *Server) {
		s.network = network
	}
}

func WithAddress(address string) Option {
	return func(s *Server) {
		s.address = address
	}
}

// NewServer 如果不指定网络类型，默认tcp；如果不指定端口，则默认7788端口
func NewServer(opts ...Option) *Server {
	s := &Server{
		network: "tcp",
		address: ":7788",
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
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
			log.Printf("decode connect settings fail: %s\n", err)
			continue
		}
		// 判断是不是toyrpc的连接，不是的话直接关闭，打印错误日志
		if settings.MagicNumber != MagicNumber {
			_ = netConn.Close()
			log.Println("unknown message type")
			continue
		}
		// 获取编码类型
		maker, err := codec.Get(settings.CodecType)
		if err != nil {
			_ = netConn.Close()
			log.Println("unknown encoding type")
			continue
		}
		// 新建toyrpc连接
		conn := codec.NewConn(maker(netConn))
		// 是toyrpc连接则处理
		go conn.Handle()
	}
}
