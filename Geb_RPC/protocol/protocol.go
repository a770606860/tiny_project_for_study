package protocol

import (
	"gebrpc/codec"
)

const MagicNumber = 0x3bef5

type Option struct {
	MagicNumber int
	CodecType   codec.Type
}

type ServerReply struct {
	Code string
	Tick int64 // 心跳间隔，单位秒
}

var DefaultOption = &Option{
	MagicNumber: MagicNumber,
	CodecType:   codec.GobType,
}
