package toyrpc

import (
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"reflect"
	"sync"
	"time"

	"toyrpc/codec"
)

// Connection 一个连接上，字节流的格式是
// Settings | Call1 header | Call1 body | Call2 header | Call2 body | ...
type Connection struct {
	codec.Codec             // 一个net.Conn对应一个Codec
	sending     *sync.Mutex // 多个调用的reply在一个套接字上发送，为了保证每一个reply都连续完整，发送时候需要加锁
	wg          *sync.WaitGroup
}

type Request struct {
	H     *codec.Header
	Args  reflect.Value
	Reply reflect.Value
}

func NewConn(cd codec.Codec) *Connection {
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
		req := &Request{H: new(codec.Header)}
		// 1 解析请求头
		if err := conn.ReadHeader(req.H); err != nil {
			log.Printf("Connection.Codec read header fail: %s\n", err)
			if err != io.EOF && !errors.Is(err, io.ErrUnexpectedEOF) {
				req.H.Err = err.Error()
				conn.sendResponse(req)
			}
			break // 解析失败将关闭当前连接
		}
		// 2 解析请求参数（body）
		req.Args = reflect.New(reflect.TypeOf(""))
		if err := conn.ReadBody(req.Args.Interface()); err != nil {
			log.Printf("Connection.Codec read body fail: %s\n", err)
			req.H.Err = err.Error()
			conn.sendResponse(req)
			break // 解析失败将关闭当前连接
		}
		// 3 交给一个goroutine完成调用
		conn.wg.Add(1)
		go func() {
			defer conn.wg.Done()
			err := conn.doCall(req)
			if err != nil {
				log.Println(err)
				req.H.Err = err.Error()
				conn.sendResponse(req)
			}
		}()
	}
	// 保证如果出错要关闭连接 也应该等待已经发出调用的goroutine返回
	conn.wg.Wait()
}

func (conn *Connection) sendResponse(req *Request) {
	conn.sending.Lock()
	defer conn.sending.Unlock()
	var body any
	// 判断reply是否有效 如果发生了错误 reply应该是个无效的零值
	if req.Reply.IsValid() {
		// 如果有效才调用Interface() 否则会panic
		body = req.Reply.Interface()
	}
	if err := conn.Write(req.H, body); err != nil {
		log.Printf("Connection.Codec write fail: %s\n", err)
	}
}

func (conn *Connection) doCall(req *Request) error {
	time.Sleep(time.Duration(rand.Intn(5)) * time.Second)
	fmt.Println(req.H.Service, req.H.Method)
	fmt.Println(req.Args.Elem().String())
	req.Reply = reflect.ValueOf("this is resp from server:" + req.Args.Elem().String())
	conn.sendResponse(req)
	return nil
}
