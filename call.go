package toyrpc

type Call struct {
	*Request
	Done    chan struct{}
	Err     error
	Invalid bool
}

func (c *Call) done() {
	c.Done <- struct{}{}
}
