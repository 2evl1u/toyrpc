package test

import (
	"fmt"
	"testing"

	"toyrpc"
)

func TestServer(t *testing.T) {
	svr := toyrpc.NewServer(toyrpc.WithSvrAddress(":7788"))
	err := svr.AsService(&Adder{})
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
	if err := cli.Call("Adder", "DoComplex", args, ret); err != nil {
		fmt.Println("cli.Call fail:", err)
	}
	fmt.Println("ret:", ret)
}