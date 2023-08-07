package toyrpc

import (
	"io"
	"reflect"
	"strings"
	"sync"

	. "toyrpc/log"

	"toyrpc/codec"

	"github.com/pkg/errors"
)

// Connection 一个连接上，字节流的格式是
// Settings | Call1 header | Call1 body | Call2 header | Call2 body | ...
type connection struct {
	codec.Codec             // 一个net.Conn对应一个Codec
	sending     *sync.Mutex // 多个调用的reply在一个套接字上发送，为了保证每一个reply都连续完整，发送时候需要加锁
	wg          *sync.WaitGroup
	svr         *Server
}

type request struct {
	h     *codec.Header
	args  reflect.Value
	reply reflect.Value
}

// Handle 接手一个套接字连接
func (conn *connection) handle() {
	defer func() {
		_ = conn.Close()
	}()
	for {
		req := &request{h: new(codec.Header)}
		// 1 解析请求头
		if err := conn.ReadHeader(req.h); err != nil {
			if err != io.EOF && !errors.Is(err, io.ErrUnexpectedEOF) &&
				!strings.Contains(err.Error(), "An existing connection was forcibly closed by the remote host") {
				ErrorLogger.Printf("Connection.Codec read header fail: %s\n", err)
				req.h.Err = err.Error()
				conn.sendResponse(req)
			}
			ErrorLogger.Printf("Connection is closed: %s\n", err)
			break // 解析失败将关闭当前连接
		}
		// 2 解析请求参数（body）
		// 加载对应服务与方法
		s, ok := conn.svr.serviceMap.Load(req.h.Service)
		if !ok {
			ErrorLogger.Printf("Service %s doesn't exist\n", req.h.Service)
			break
		}
		svc := s.(*service)
		method, ok := svc.mm[req.h.Method]
		if !ok {
			ErrorLogger.Printf("Method %s doesn't exist\n", req.h.Method)
			break
		}
		argT, replyT := method.Type.In(1), method.Type.In(2)
		req.args, req.reply = newArgv(argT), newReplyv(replyT)
		// 这里是因为如果arg不是指针类型，需要拿到其指针才能用于下面ReadBody的读取
		argPtr := req.args.Interface()
		if req.args.Type().Kind() != reflect.Ptr {
			argPtr = req.args.Addr().Interface()
		}
		if err := conn.ReadBody(argPtr); err != nil {
			ErrorLogger.Printf("Connection.Codec read body fail: %s\n", err)
			req.h.Err = err.Error()
			conn.sendResponse(req)
			break // 解析失败将关闭当前连接
		}
		// 3 交给一个goroutine完成调用
		conn.wg.Add(1)
		go func() {
			defer conn.wg.Done()
			if err := conn.doCall(req); err != nil {
				ErrorLogger.Printf("Call %s.%s fail: %s\n", req.h.Service, req.h.Method, err)
				req.h.Err = err.Error()
				conn.sendResponse(req)
			}
		}()
	}
	// 保证如果出错要关闭连接 也应该等待已经发出调用的goroutine返回
	conn.wg.Wait()
}

func (conn *connection) sendResponse(req *request) {
	conn.sending.Lock()
	defer conn.sending.Unlock()
	var body any
	// 判断reply是否有效 如果发生了错误 reply应该是个无效的零值
	if req.reply.IsValid() {
		// 如果有效才调用Interface() 否则会panic
		body = req.reply.Interface()
	}
	if err := conn.Write(req.h, body); err != nil {
		ErrorLogger.Printf("Connection.Codec write fail: %s\n", err)
	}
}

// 加载对应的服务并调用
func (conn *connection) doCall(req *request) error {
	s, _ := conn.svr.serviceMap.Load(req.h.Service)
	svc := s.(*service)
	method := svc.mm[req.h.Method]
	ret := method.Func.Call([]reflect.Value{svc.self, req.args, req.reply})
	if err := ret[0].Interface(); err != nil {
		return err.(error)
	}
	conn.sendResponse(req)
	return nil
}

func newArgv(t reflect.Type) reflect.Value {
	var argv reflect.Value
	if t.Kind() == reflect.Ptr {
		argv = reflect.New(t.Elem())
	} else {
		argv = reflect.New(t).Elem()
	}
	return argv
}

func newReplyv(t reflect.Type) reflect.Value {
	// reply must be a pointer type
	replyv := reflect.New(t.Elem())
	switch t.Elem().Kind() {
	case reflect.Map:
		replyv.Elem().Set(reflect.MakeMap(t.Elem()))
	case reflect.Slice:
		replyv.Elem().Set(reflect.MakeSlice(t.Elem(), 0, 0))
	}
	return replyv
}
