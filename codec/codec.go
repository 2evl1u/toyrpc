package codec

import (
	"errors"
	"fmt"
	"io"
	"sync"
)

type Header struct {
	Service string
	Method  string
	SeqId   uint64
	Err     error
}

type Codec interface {
	io.Closer
	ReadHeader(header *Header) error
	ReadBody(body any) error
	Write(h *Header, body any) error
}

const (
	GobType  = "gob"
	JSONType = "json"
)

type Maker func(conn io.ReadWriteCloser) Codec

type typeMap struct {
	m  map[string]Maker
	mu *sync.RWMutex // 用于保护前一个属性m
}

var defaultTypeMap = &typeMap{
	m: map[string]Maker{
		GobType:  NewGobEncDec,
		JSONType: NewJSONEncDec,
	},
	mu: new(sync.RWMutex),
}

func (tm *typeMap) get(typeName string) (Maker, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	coderMaker, ok := tm.m[typeName]
	if !ok {
		return nil, errors.New(fmt.Sprintf("inexistent codec type name: %s\n", typeName))
	}
	return coderMaker, nil
}

func (tm *typeMap) register(typeName string, maker Maker) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	maker, ok := tm.m[typeName]
	if !ok {
		return errors.New(fmt.Sprintf("codec with the name [%s] already exists", typeName))
	}
	tm.m[typeName] = maker
	return nil
}

// Get 获取相应名字的编解码器生成器
func Get(typeName string) (Maker, error) {
	return defaultTypeMap.get(typeName)
}

// Register 用于注册一个编解码器生成器到toyrpc中
func Register(typeName string, maker Maker) error {
	return defaultTypeMap.register(typeName, maker)
}
