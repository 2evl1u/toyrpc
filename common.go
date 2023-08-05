package toyrpc

import (
	"time"

	"toyrpc/codec"
)

const MagicNumber = 0x3bef5c

const DefaultAddr = ":7788"

const DefaultNetwork = "tcp"

const DefaultCodec = "json"

// DefaultServerHeartbeatInterval 心跳的间隔比服务器超时间隔稍短
const DefaultServerHeartbeatInterval = DefaultTimeoutInterval - time.Minute

var DefaultSettings = Settings{
	MagicNumber: MagicNumber,
	CodecType:   codec.JSONType,
}

type Settings struct {
	MagicNumber int
	CodecType   string
}
