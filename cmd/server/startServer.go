package main

import "toyrpc"

func main() {
	s := toyrpc.NewServer(toyrpc.WithAddress(":6789"))
	s.Start()
}
