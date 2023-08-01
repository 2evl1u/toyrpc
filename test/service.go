package test

import "github.com/pkg/errors"

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
	*ret = UserRet{
		UUID:    "JKFLSDHFJQEUI",
		Address: "CHINA",
		Status: []Args{
			{3, 4},
			{6, 8},
		},
	}
	return errors.New("a call error")
}
