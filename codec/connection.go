package codec

import (
	"fmt"
	"io"
	"log"
	"reflect"
	"sync"
)

// Connection 一个连接上，字节流的格式是
// Settings | Call1 header | Call1 body | Call2 header | Call2 body | ...
type Connection struct {
	Codec               // 一个net.Conn对应一个Codec
	sending *sync.Mutex // 多个调用的reply在一个套接字上发送，为了保证每一个reply都连续完整，发送时候需要加锁
	wg      *sync.WaitGroup
}

type Request struct {
	H     *Header
	Args  reflect.Value
	Reply reflect.Value
}

func NewConn(cd Codec) *Connection {
	conn := &Connection{
		Codec:   cd,
		sending: new(sync.Mutex),
		wg:      new(sync.WaitGroup),
	}
	return conn
}

func (conn *Connection) Handle() {
	defer func() {
		_ = conn.Close()
	}()
	for {
		req := &Request{H: new(Header)}
		// 1 读取请求头
		if err := conn.ReadHeader(req.H); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				log.Printf("Remote client has closed: %s\n", err)
				break
			}
			log.Printf("Connection.Codec read header fail: %s\n", err)
			req.Reply = reflect.ValueOf(fmt.Sprintf("Read header fail: %s\n", err))
			conn.sendResponse(req)
			continue
		}
		// 2 读取请求参数（body）
		req.Args = reflect.New(reflect.TypeOf(""))
		if err := conn.ReadBody(req.Args.Interface()); err != nil {
			log.Printf("Connection.Codec read body fail: %s\n", err)
			req.Reply = reflect.ValueOf(fmt.Sprintf("Read Args fail: %s\n", err))
			conn.sendResponse(req)
		}
		// 3 交给一个goroutine完成调用
		// go DoCall(req)
		fmt.Println(req.H.Service, req.H.Method)
		fmt.Println(req.Args.Elem().String())
		req.Reply = reflect.ValueOf("this is resp from server")
		fmt.Println(req.Reply.Interface())
		conn.sendResponse(req)
		conn.wg.Add(1)
	}
	conn.wg.Wait()
}

func (conn *Connection) sendResponse(req *Request) {
	conn.sending.Lock()
	defer conn.sending.Unlock()
	if err := conn.Write(req.H, req.Reply.Interface()); err != nil {
		log.Printf("Connection.Codec write fail: %s\n", err)
	}
}
