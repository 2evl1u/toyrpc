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
	time.Sleep(3 * time.Second)
	json.NewEncoder(conn).Encode(toyrpc.Settings{
		MagicNumber: toyrpc.MagicNumber,
		CodecType:   "gob",
	})
	maker, _ := codec.Get("gob")
	cd := maker(conn)
	req := &codec.Request{
		H: &codec.Header{
			Service: "TestService",
			Method:  "TestMethod",
			SeqId:   123,
			Err:     nil,
		},
		Args:  reflect.ValueOf("this is a call"),
		Reply: reflect.New(reflect.TypeOf("")),
	}
	cd.Write(req.H, req.Args.Interface())
	fmt.Println(req.Args.String())
	cd.ReadHeader(req.H)
	err := cd.ReadBody(req.Reply.Interface())
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(req.Reply.Elem().Interface())
	defer conn.Close()
}
