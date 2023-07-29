package main

import (
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"time"

	"toyrpc"
	"toyrpc/codec"
)

func main() {
	// 注册自身的服务

	// 调用服务
	// cli, err := toyrpc.Dial("tcp", ":6789")
	// if err != nil {
	// 	log.Printf("toyrpc dail server fail: %s\n", err)
	// }
	// defer cli.Close()
	// var wg sync.WaitGroup
	// for i := 0; i < 5; i++ {
	// 	wg.Add(1)
	// 	go func(i int) {
	// 		defer wg.Done()
	// 		var arg Args{}
	// 		var reply Reply{}
	// 		err = cli.Call("ServiceX.MethodX", arg, &reply)
	// 		if err != nil {
	// 			log.Printf("call fail %s\n", err)
	// 		}
	// 	}(i)
	// }
	// wg.Wait()

	conn, _ := net.Dial("tcp", ":6789")
	defer conn.Close()
	time.Sleep(time.Second)
	json.NewEncoder(conn).Encode(toyrpc.Settings{
		MagicNumber: toyrpc.MagicNumber,
		CodecType:   "json",
	})
	maker, _ := codec.Get("json")
	cd := maker(conn)
	go func() {
		for i := 0; i < 10; i++ {
			req := &toyrpc.Request{
				H: &codec.Header{
					Service: "TestService",
					Method:  "TestMethod",
					SeqId:   uint64(i),
					Err:     "",
				},
				Args: reflect.ValueOf(fmt.Sprintf("this is a call %d", i)),
			}
			err := cd.Write(req.H, req.Args.Interface())
			if err != nil {
				fmt.Printf("Write fail: %s\n", err)
			}
		}
	}()
	req := &toyrpc.Request{
		H:     &codec.Header{},
		Reply: reflect.New(reflect.TypeOf("")),
	}
	for {
		err := cd.ReadHeader(req.H)
		if err != nil {
			fmt.Printf("Read header fail: %s\n", err)
			break
		}
		if req.H.Err != "" {
			fmt.Println(req.H.Err)
		}
		err = cd.ReadBody(req.Reply.Interface())
		if err != nil {
			fmt.Printf("Read body fail: %s\n", err)
		}
		fmt.Println("Server reply:", req.Reply.Elem().Interface())
	}
}
