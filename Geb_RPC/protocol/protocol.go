package protocol

import (
	"gebrpc/codec"
)

const MagicNumber = 0x3bef5

type Option struct {
	MagicNumber int
	CodecType   codec.Type
}

var DefaultOption = &Option{
	MagicNumber: MagicNumber,
	CodecType:   codec.GobType,
}
