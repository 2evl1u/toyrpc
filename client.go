package toyrpc

import (
	"net"
	"sync"

	"toyrpc/codec"
)

type Client struct {
	codec.Codec
	network  string
	address  string
	settings *Settings
	seq      uint64
	pending  map[uint64]*Call
	sending  *sync.Mutex
}

type CliOption func(cli *Client)

func WithCliNetwork(network string) CliOption {
	return func(cli *Client) {
		cli.network = network
	}
}

func WithCliAddress(address string) CliOption {
	return func(cli *Client) {
		cli.address = address
	}
}

func WithCliCodecType(codecType string) CliOption {
	return func(cli *Client) {
		cli.settings.CodecType = codecType
	}
}

func NewClient(opts ...CliOption) *Client {
	cli := &Client{
		network:  DefaultNetwork,
		address:  DefaultAddr,
		seq:      1,
		settings: &DefaultSettings,
		pending:  make(map[uint64]*Call),
		sending:  new(sync.Mutex),
	}
	for _, opt := range opts {
		opt(cli)
	}
	maker, _ := codec.Get(cli.settings.CodecType)
	conn, err := net.Dial(cli.network, cli.address)
	if err != nil {
		panic(err)
	}
	cli.Codec = maker(conn)
	return cli
}

type Call struct {
}

func (cli *Client) receive() {
	var err error
	var h codec.Header
	for err == nil {
		if err = cli.ReadHeader(&h); err != nil {
			break
		}
		call := cli.pending[h.SeqId]
	}
}
