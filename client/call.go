package client

import "toyrpc"

type Call struct {
	*toyrpc.Request
	Done    chan struct{}
	Err     error
	Invalid bool
}

func (c *Call) done() {
	c.Done <- struct{}{}
}
