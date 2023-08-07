package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"toyrpc"
)

func TestRegistry(t *testing.T) {
	r := toyrpc.NewRegistry(toyrpc.WithPort(":9999"))
	r.Start()
}

func TestServer(t *testing.T) {
	svr := toyrpc.NewServer("http://localhost:9999", toyrpc.WithSvrAddress(":7798"))
	err := svr.AsService(&Adder{})
	if err != nil {
		fmt.Println("AsService fail:", err)
	}
	err = svr.AsService(&ErrService{})
	if err != nil {
		fmt.Println("AsService fail:", err)
	}
	svr.Start()
}

func TestClient(t *testing.T) {
	cli := toyrpc.NewClient("http://localhost:9999")

	t.Run("success", func(t *testing.T) {
		args := UserReq{
			UserId:   10045,
			UserName: "Zev",
			Married:  true,
			Param:    Args{A: 11, B: 5},
		}
		var ret = new(UserResp)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := cli.Call(ctx, "Adder", "DoComplex", args, ret); err != nil {
			fmt.Println("Call fail:", err)
		} else {
			fmt.Println("ret:", ret)
		}
		cancel()
	})

	t.Run("success2", func(t *testing.T) {
		args := Args{
			A: 3,
			B: 5,
		}
		var ret = new(int)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := cli.Call(ctx, "Adder", "Add", args, ret); err != nil {
			fmt.Println("Call fail:", err)
		} else {
			fmt.Println("ret:", *ret)
		}
		cancel()
	})

	t.Run("call fail", func(t *testing.T) {
		args := UserReq{
			UserId:   10045,
			UserName: "Zev",
			Married:  true,
			Param:    Args{A: 11, B: 5},
		}
		var ret = new(UserResp)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := cli.Call(ctx, "ErrService", "GetErr", args, ret); err != nil {
			fmt.Println("Call fail:", err)
		} else {
			fmt.Println("ret:", ret)
		}
		cancel()
	})

	t.Run("call fail2", func(t *testing.T) {
		args := UserReq{
			UserId:   10045,
			UserName: "Zev",
			Married:  true,
			Param:    Args{A: 11, B: 5},
		}
		var ret = new(int) // 错误的接收者
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := cli.Call(ctx, "Adder", "DoComplex", args, ret); err != nil {
			fmt.Println("Call fail:", err)
		} else {
			fmt.Println("ret:", ret)
		}
		cancel()
	})
}
