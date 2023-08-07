package test

import (
	"fmt"

	"github.com/pkg/errors"
)

type Args struct {
	A, B int
}

type Adder struct{}

func (a *Adder) Add(args Args, sum *int) error {
	*sum = args.A + args.B
	return nil
}

type UserReq struct {
	UserId   int
	UserName string
	Married  bool
	Param    Args
}

type UserResp struct {
	UUID    string
	Address string
	Status  []Args
}

func (a *Adder) DoComplex(userInfo UserReq, ret *UserResp) error {
	fmt.Println(userInfo)
	*ret = UserResp{
		UUID:    "ABCD-ABCD-ABCD-ABCD-ABCD",
		Address: "CHINA",
		Status: []Args{
			{3, 4},
			{6, 8},
		},
	}
	return nil
}

type ErrService struct{}

func (e *ErrService) GetErr(userInfo UserReq, ret *UserResp) error {
	return errors.New("a unexpected error")
}
