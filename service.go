package toyrpc

import "reflect"

type service struct {
	name string
	self reflect.Value
	mm   map[string]*reflect.Method
}
