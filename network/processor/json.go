package processor

/**
  *  @author tryao
  *  @date 2022/03/21 14:40
**/
import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/base/structs/rpc"
	"reflect"
)

type JsonProcessor struct {
	msgInfo map[string]*MsgInfo
}

type MsgInfo struct {
	msgType       reflect.Type
	msgRouter     rpc.IServer
	msgHandler    MsgHandler
	msgRawHandler MsgHandler
}

type MsgHandler func([]any)

type MsgRaw struct {
	msgID      string
	msgRawData json.RawMessage
}

func NewProcessor() *JsonProcessor {
	p := new(JsonProcessor)
	p.msgInfo = make(map[string]*MsgInfo)
	return p
}

// Register 注册消息
//It's dangerous to call the method on routing or
//marshaling/unmarshalling
func (p *JsonProcessor) Register(msg any) string {
	msgType := reflect.TypeOf(msg)
	if msgType == nil || msgType.Kind() != reflect.Ptr {
		log.Fatal("json message pointer required")
	}
	msgID := msgType.Elem().Name()
	if msgID == "" {
		log.Fatal("unnamed json message")
	}
	if _, ok := p.msgInfo[msgID]; ok {
		log.Fatal("message %v is already registered", msgID)
	}

	i := new(MsgInfo)
	i.msgType = msgType
	p.msgInfo[msgID] = i
	return msgID
}

// SetRouter 设置路由
//It's dangerous to call the method on routing or marshaling/unmarshalling
func (p *JsonProcessor) SetRouter(msg any, msgRouter rpc.IServer) {
	msgType := reflect.TypeOf(msg)
	if msgType == nil || msgType.Kind() != reflect.Ptr {
		log.Fatal("json message pointer required")
	}
	msgID := msgType.Elem().Name()
	i, ok := p.msgInfo[msgID]
	if !ok {
		log.Fatal("message %v not registered", msgID)
	}

	i.msgRouter = msgRouter
}

// SetHandler 直接设置回调处理
//It's dangerous to call the method on routing or marshaling (unmarshaling)
func (p *JsonProcessor) SetHandler(msg any, msgHandler MsgHandler) {
	msgType := reflect.TypeOf(msg)
	if msgType == nil || msgType.Kind() != reflect.Ptr {
		log.Fatal("json message pointer required")
	}
	msgID := msgType.Elem().Name()
	i, ok := p.msgInfo[msgID]
	if !ok {
		log.Fatal("message %v not registered", msgID)
	}

	i.msgHandler = msgHandler
}

// SetRawHandler 原始数据处理
//It's dangerous to call the method on routing or marshaling (unmarshaling)
func (p *JsonProcessor) SetRawHandler(msgID string, msgRawHandler MsgHandler) {
	i, ok := p.msgInfo[msgID]
	if !ok {
		log.Fatal("message %v not registered", msgID)
	}

	i.msgRawHandler = msgRawHandler
}

func (p *JsonProcessor) Route(msg any, userData any) error {
	// raw
	if msgRaw, ok := msg.(MsgRaw); ok {
		i, ok := p.msgInfo[msgRaw.msgID]
		if !ok {
			return fmt.Errorf("message %v not registered", msgRaw.msgID)
		}
		if i.msgRawHandler != nil {
			i.msgRawHandler([]any{msgRaw.msgID, msgRaw.msgRawData, userData})
		}
		return nil
	}

	// json
	msgType := reflect.TypeOf(msg)
	if msgType == nil || msgType.Kind() != reflect.Ptr {
		return errors.New("json message pointer required")
	}
	msgID := msgType.Elem().Name()
	i, ok := p.msgInfo[msgID]
	if !ok {
		return fmt.Errorf("message %v not registered", msgID)
	}
	if i.msgHandler != nil {
		i.msgHandler([]any{msg, userData})
	}
	if i.msgRouter != nil {
		i.msgRouter.Go(msgType, msg, userData)
	}
	return nil
}

func (p *JsonProcessor) Unmarshal(data []byte) (any, error) {
	var m map[string]json.RawMessage
	err := json.Unmarshal(data, &m)
	if err != nil {
		return nil, err
	}
	if len(m) != 1 {
		return nil, errors.New("invalid json data")
	}

	for msgID, data := range m {
		i, ok := p.msgInfo[msgID]
		if !ok {
			return nil, fmt.Errorf("message %v not registered", msgID)
		}

		// msg
		if i.msgRawHandler != nil {
			return MsgRaw{msgID, data}, nil
		} else {
			msg := reflect.New(i.msgType.Elem()).Interface()
			return msg, json.Unmarshal(data, msg)
		}
	}

	panic("bug")
}

func (p *JsonProcessor) Marshal(msg any) ([][]byte, error) {
	msgType := reflect.TypeOf(msg)
	if msgType == nil || msgType.Kind() != reflect.Ptr {
		return nil, errors.New("json message pointer required")
	}
	msgID := msgType.Elem().Name()
	if _, ok := p.msgInfo[msgID]; !ok {
		return nil, fmt.Errorf("message %v not registered", msgID)
	}

	// data
	m := map[string]any{msgID: msg}
	data, err := json.Marshal(m)
	return [][]byte{data}, err
}
