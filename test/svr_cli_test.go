package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"toyrpc"
	"toyrpc/client"
)

func TestRegistry(t *testing.T) {

}

func TestServer(t *testing.T) {
	svr := toyrpc.NewServer(toyrpc.WithSvrAddress(":7799"))
	err := svr.AsService(&Adder{})
	if err != nil {
		fmt.Println("AsService fail:", err)
	}
	svr.Start()
}

func TestClient(t *testing.T) {
	// :7788 是注册中心地址
	cli := client.NewClient(":7788")
	args := UserInfo{
		UserId:   10045,
		UserName: "Zev",
		Married:  true,
		Param:    Args{A: 11, B: 5},
	}
	var ret = new(UserRet)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := cli.Call(ctx, "Adder", "DoComplex", args, ret); err != nil {
		fmt.Println("cli.Call fail:", err)
	}
	fmt.Println("ret:", ret)
}
