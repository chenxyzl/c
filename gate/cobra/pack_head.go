package cobra

import (
	"github.com/topfreegames/pitaya/logger"
)

type PackHead struct {
	Length uint32
	Cmd    uint32
	Uid    uint64
}

func (this *PackHead) copy(ph *PackHead) *PackHead {
	this.Length = ph.Length
	this.Cmd = ph.Cmd
	this.Uid = ph.Uid
	return this
}

//Big Endian
func DecodePackHead(buf []byte, ph *PackHead) bool {
	if len(buf) < 16 {
		logger.Log.Error("[PackHead] decode size no enough size: %v", len(buf))
		return false
	}
	ph.Length = DecodeUint32(buf[0:])
	ph.Cmd = DecodeUint32(buf[4:])
	ph.Uid = DecodeUint64(buf[8:])
	return true
}

//Big Endian
func EncodePackHead(buf []byte, ph *PackHead) bool {
	if len(buf) < 16 {
		logger.Log.Error("[PackHead] encode size no enough size: %v", len(buf))
		return false
	}
	EncodeUint32(ph.Length, buf[0:])
	EncodeUint32(ph.Cmd, buf[4:])
	EncodeUint64(ph.Uid, buf[8:])
	return true
}
