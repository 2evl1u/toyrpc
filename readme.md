# toyrpc
这是一个简单的RPC框架，实现了RPC通信最基础的功能。
本框架基于学习[GeePRC](https://geektutu.com/post/geerpc.html)之后思考实践开发，用于学习go语言以及RPC通信过程的核心问题。
代码量大约1000行。

通过下面命令安装该框架：
```bash
go get github.com/2evl1u/toyrpc
```

### 使用示例

首先需要先将注册中心启动
```go
package main

import "toyrpc"

func main() {
	r := toyrpc.NewRegistry(toyrpc.WithPort(":9999"))
	r.Start()
}
```

接着将运行提供服务的实例
```go
func main() {
	svr := toyrpc.NewServer("http://localhost:9999", toyrpc.WithSvrAddress(":7798"))
	err := svr.AsService(&Adder{})
	if err != nil {
		fmt.Println("AsService fail:", err)
	}
	svr.Start()
}
```
服务的定义单独放在`svc_def.go`中
```go
package main

import "time"

type Args struct {
	A, B int
}

type Adder struct{}

func (a *Adder) Add(args Args, sum *int) error {
	*sum = args.A + args.B
	return nil
}

type UserInfo struct {
	UserId   int
	UserName string
	Married  bool
	Param    Args
}

type UserRet struct {
	UUID    string
	Address string
	Status  []Args
}

func (a *Adder) DoComplex(userInfo UserInfo, ret *UserRet) error {
	time.Sleep(3 * time.Second)
	*ret = UserRet{
		UUID:    "JKFLSDHFJQEUI",
		Address: "CHINA",
		Status: []Args{
			{3, 4},
			{6, 8},
		},
	}
	return nil
}

```

最后运行客户端进行请求即可
```go
package main

import (
	"context"
	"fmt"
	"time"

	"toyrpc"
)

func main() {
	cli := toyrpc.NewClient("http://localhost:9999")
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
	} else {
		fmt.Println("ret:", ret)
    }
	cancel()
}
```