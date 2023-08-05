package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"toyrpc"
	"toyrpc/xclient"
)

func TestRegistry(t *testing.T) {
	r := toyrpc.NewRegistry()
	r.Start()
}

func TestServer(t *testing.T) {
	svr := toyrpc.NewServer("http://localhost:9999", toyrpc.WithSvrAddress(":7798"))
	err := svr.AsService(&Adder{})
	if err != nil {
		fmt.Println("AsService fail:", err)
	}
	svr.Start()
}

func TestClient(t *testing.T) {
	cli := xclient.NewClient("http://localhost:9999")

	args := UserInfo{
		UserId:   10045,
		UserName: "Zev",
		Married:  true,
		Param:    Args{A: 11, B: 5},
	}
	var ret = new(UserRet)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := cli.Call(ctx, "Adder", "DoComplex", args, ret); err != nil {
		fmt.Println("cli.Call fail:", err)
	}
	fmt.Println("ret:", ret)
	cancel()

	args2 := Args{
		A: 3,
		B: 5,
	}
	var ret2 = new(int)
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	if err := cli.Call(ctx, "Adder", "Add", args2, ret2); err != nil {
		fmt.Println("cli.Call fail:", err)
	}
	fmt.Println("ret2:", *ret2)
	cancel()
}
