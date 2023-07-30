package toyrpc

import (
	"toyrpc/codec"
)

const MagicNumber = 0x3bef5c

const DefaultAddr = ":7788"

const DefaultNetwork = "tcp"

const DefaultCodec = "json"

var DefaultSettings = Settings{
	MagicNumber: MagicNumber,
	CodecType:   codec.JSONType,
}

type Settings struct {
	MagicNumber int
	CodecType   string
}
