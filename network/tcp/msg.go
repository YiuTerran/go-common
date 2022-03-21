package tcp

import (
	"encoding/binary"
	"errors"
	"github.com/YiuTerran/go-common/base/log"
	"io"
	"math"
)

// BinaryParser 一个默认的二进制解析器，可以拿来做服务端封装
//--------------
// | len | data |
// --------------
type BinaryParser struct {
	lenMsgLen    int
	minMsgLen    uint32
	maxMsgLen    uint32
	littleEndian bool
}

// NewDefaultParser 默认使用2位长度标识,大端序
func NewDefaultParser() *BinaryParser {
	return NewBinaryParser(2, false)
}

func NewBinaryParser(lenMsgLen int, littleEndian bool) *BinaryParser {
	p := new(BinaryParser)
	p.littleEndian = littleEndian
	p.setMsgLen(lenMsgLen)
	return p
}

func (p *BinaryParser) setMsgLen(lenMsgLen int) {
	if lenMsgLen == 1 || lenMsgLen == 2 || lenMsgLen == 4 {
		p.lenMsgLen = lenMsgLen
	} else {
		log.Warn("invalid lenMsgLen %d, using 2", lenMsgLen)
		p.lenMsgLen = 2
	}
	var max uint32
	switch p.lenMsgLen {
	case 1:
		max = math.MaxUint8
	case 2:
		max = math.MaxUint16
	case 4:
		max = math.MaxUint32
	}
	p.minMsgLen = 1
	p.maxMsgLen = max
}

// Read 从连接中读取数据，goroutine safe
func (p *BinaryParser) Read(conn *Conn) ([]byte, error) {
	var b [4]byte
	bufMsgLen := b[:p.lenMsgLen]

	// read len
	if _, err := io.ReadFull(conn, bufMsgLen); err != nil {
		return nil, err
	}

	// parse len
	var msgLen uint32
	switch p.lenMsgLen {
	case 1:
		msgLen = uint32(bufMsgLen[0])
	case 2:
		if p.littleEndian {
			msgLen = uint32(binary.LittleEndian.Uint16(bufMsgLen))
		} else {
			msgLen = uint32(binary.BigEndian.Uint16(bufMsgLen))
		}
	case 4:
		if p.littleEndian {
			msgLen = binary.LittleEndian.Uint32(bufMsgLen)
		} else {
			msgLen = binary.BigEndian.Uint32(bufMsgLen)
		}
	}

	// check len
	if msgLen > p.maxMsgLen {
		return nil, errors.New("message too long")
	} else if msgLen < p.minMsgLen {
		return nil, errors.New("message too short")
	}

	// data
	msgData := make([]byte, msgLen)
	if _, err := io.ReadFull(conn, msgData); err != nil {
		return nil, err
	}

	return msgData, nil
}

// Write 向连接中写入数据，goroutine safe
func (p *BinaryParser) Write(conn *Conn, args ...[]byte) error {
	// get len
	var msgLen uint32
	for i := 0; i < len(args); i++ {
		msgLen += uint32(len(args[i]))
	}

	// check len
	if msgLen > p.maxMsgLen {
		return errors.New("message too long")
	} else if msgLen < p.minMsgLen {
		return errors.New("message too short")
	}

	msg := make([]byte, uint32(p.lenMsgLen)+msgLen)

	// write len
	switch p.lenMsgLen {
	case 1:
		msg[0] = byte(msgLen)
	case 2:
		if p.littleEndian {
			binary.LittleEndian.PutUint16(msg, uint16(msgLen))
		} else {
			binary.BigEndian.PutUint16(msg, uint16(msgLen))
		}
	case 4:
		if p.littleEndian {
			binary.LittleEndian.PutUint32(msg, msgLen)
		} else {
			binary.BigEndian.PutUint32(msg, msgLen)
		}
	}

	// write data
	l := p.lenMsgLen
	for i := 0; i < len(args); i++ {
		copy(msg[l:], args[i])
		l += len(args[i])
	}

	conn.Write(msg)

	return nil
}
