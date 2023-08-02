package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"toyrpc"
)

func TestServer(t *testing.T) {
	svr := toyrpc.NewServer(toyrpc.WithSvrAddress(":7788"))
	err := svr.AsService(&Adder{}, 2*time.Second)
	if err != nil {
		fmt.Println("AsService fail:", err)
	}
	svr.Start()
}

func TestClient(t *testing.T) {
	cli := toyrpc.NewClient(":7788")
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
