package toyrpc

type Call struct {
	*Request
	Done chan struct{}
	Err  error
}

func (c *Call) done() {
	c.Done <- struct{}{}
}
