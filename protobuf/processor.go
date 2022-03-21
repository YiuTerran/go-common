package protobuf

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/base/structs/rpc"
	"google.golang.org/protobuf/proto"
	"math"
	"reflect"
)

// -------------------------
// | id | protobuf message |
// -------------------------
type processor struct {
	littleEndian bool
	msgInfo      map[uint16]*msgInfoST
	msgID        map[reflect.Type]uint16
}

type msgInfoST struct {
	msgType       reflect.Type
	msgRouter     rpc.IServer
	msgHandler    msgHandlerST
	msgRawHandler msgHandlerST
}

type msgHandlerST func([]any)

type msgRawST struct {
	msgID      uint16
	msgRawData []byte
}

func NewProcessor(littleEndian bool) *processor {
	p := new(processor)
	p.littleEndian = littleEndian
	p.msgID = make(map[reflect.Type]uint16)
	p.msgInfo = make(map[uint16]*msgInfoST)
	return p
}

// Register 注册消息
// It's dangerous to call the method on routing or marshaling/unmarshalling
func (p *processor) Register(msg proto.Message, eventType uint16) {
	msgType := reflect.TypeOf(msg)
	if msgType == nil || msgType.Kind() != reflect.Ptr {
		log.Fatal("protobuf message pointer required")
	}
	if _, ok := p.msgID[msgType]; ok {
		log.Fatal("message %s is already registered", msgType)
	}
	if len(p.msgInfo) >= math.MaxUint16 {
		log.Fatal("too many protobuf messages (max = %v)", math.MaxUint16)
	}

	i := new(msgInfoST)
	i.msgType = msgType
	p.msgInfo[eventType] = i
	p.msgID[msgType] = eventType
}

// SetRouter 设置路由
//It's dangerous to call the method on routing or marshaling/unmarshalling
func (p *processor) SetRouter(msg proto.Message, msgRouter rpc.IServer) {
	msgType := reflect.TypeOf(msg)
	id, ok := p.msgID[msgType]
	if !ok {
		log.Fatal("message %s not registered", msgType)
	}

	p.msgInfo[id].msgRouter = msgRouter
}

// SetHandler 直接设置回调处理
//It's dangerous to call the method on routing or marshaling (unmarshaling)
func (p *processor) SetHandler(msg proto.Message, msgHandler msgHandlerST) {
	msgType := reflect.TypeOf(msg)
	id, ok := p.msgID[msgType]
	if !ok {
		log.Fatal("message %s not registered", msgType)
	}

	p.msgInfo[id].msgHandler = msgHandler
}

// SetRawHandler 设置原始数据handler
//It's dangerous to call the method on routing or marshaling/unmarshalling
func (p *processor) SetRawHandler(id uint16, msgRawHandler msgHandlerST) {
	if id >= uint16(len(p.msgInfo)) {
		log.Fatal("message id %v not registered", id)
	}

	p.msgInfo[id].msgRawHandler = msgRawHandler
}

func (p *processor) Route(msg any, userData any) error {
	// raw
	if msgRaw, ok := msg.(msgRawST); ok {
		if msgRaw.msgID >= uint16(len(p.msgInfo)) {
			return fmt.Errorf("message id %v not registered", msgRaw.msgID)
		}
		i := p.msgInfo[msgRaw.msgID]
		if i.msgRawHandler != nil {
			i.msgRawHandler([]any{msgRaw.msgID, msgRaw.msgRawData, userData})
		}
		return nil
	}

	// protobuf
	msgType := reflect.TypeOf(msg)
	id, ok := p.msgID[msgType]
	if !ok {
		return fmt.Errorf("message %s not registered", msgType)
	}
	i := p.msgInfo[id]
	if i.msgHandler != nil {
		i.msgHandler([]any{msg, userData})
	}
	if i.msgRouter != nil {
		i.msgRouter.Go(msgType, msg, userData)
	}
	return nil
}

func (p *processor) Unmarshal(data []byte) (any, error) {
	if len(data) < 2 {
		return nil, errors.New("protobuf data too short")
	}

	// id
	var id uint16
	if p.littleEndian {
		id = binary.LittleEndian.Uint16(data)
	} else {
		id = binary.BigEndian.Uint16(data)
	}
	i, ok := p.msgInfo[id]
	if !ok {
		return nil, fmt.Errorf("message id %v not registered", id)
	}
	// msg
	if i.msgRawHandler != nil {
		return msgRawST{id, data[2:]}, nil
	} else {
		msg := reflect.New(i.msgType.Elem()).Interface()
		return msg, proto.Unmarshal(data[2:], msg.(proto.Message))
	}
}

func (p *processor) Marshal(msg any) ([][]byte, error) {
	msgType := reflect.TypeOf(msg)

	// id
	_id, ok := p.msgID[msgType]
	if !ok {
		err := fmt.Errorf("message %s not registered", msgType)
		return nil, err
	}

	id := make([]byte, 2)
	if p.littleEndian {
		binary.LittleEndian.PutUint16(id, _id)
	} else {
		binary.BigEndian.PutUint16(id, _id)
	}

	// data
	data, err := proto.Marshal(msg.(proto.Message))
	return [][]byte{id, data}, err
}

func (p *processor) Range(f func(id uint16, t reflect.Type)) {
	for id, i := range p.msgInfo {
		f(id, i.msgType)
	}
}
