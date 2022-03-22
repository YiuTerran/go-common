package gate

import (
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/network"
	"net"
	"reflect"
)

//Agent 是对各种网络协议连接的抽象
//与Session配合
type Agent interface {
	WriteMsg(msg any)
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
	Close()
	Destroy()
	UserData() any
	SetUserData(data any)
}

// SessionAgentImpl 满足Session和Agent接口的默认实现
type SessionAgentImpl struct {
	Conn network.Conn
	Gate IGate
	Data any
}

// CheckAuth 一般的用来校验是否验证通过的函数
func CheckAuth(ag Agent) bool {
	if ag == nil {
		return false
	}
	if ag.UserData() == nil {
		ag.Close()
		return false
	}
	return true
}

// Run session数据的处理循环
//这里出现真的错误才要断开连接
func (a *SessionAgentImpl) Run() {
	for {
		data, err := a.Conn.ReadMsg()
		if err != nil {
			log.Debug("read message error: %v", err)
			break
		}
		if len(data) == 0 {
			continue
		}
		if a.Gate.Processor() != nil {
			msg, err := a.Gate.Processor().Unmarshal(data)
			if err != nil {
				log.Debug("unmarshal message error: %v", err)
				break
			}
			if msg == nil {
				continue
			}
			err = a.Gate.Processor().Route(msg, a)
			if err != nil {
				log.Debug("route message error: %v", err)
				break
			}
		}
	}
}

func (a *SessionAgentImpl) OnClose() {
	if a.Gate.AgentChanRPC() != nil {
		err := a.Gate.AgentChanRPC().Call0(AgentBeforeCloseEvent, a)
		if err != nil {
			log.Warn("chanrpc error: %v", err)
		}
	}
}

func (a *SessionAgentImpl) WriteMsg(msg any) {
	if a.Gate.Processor() != nil {
		data, err := a.Gate.Processor().Marshal(msg)
		if err != nil {
			log.Error("marshal message %v error: %v", reflect.TypeOf(msg), err)
			return
		}
		err = a.Conn.WriteMsg(data...)
		if err != nil {
			log.Error("write message %v error: %v", reflect.TypeOf(msg), err)
		}
	}
}

func (a *SessionAgentImpl) LocalAddr() net.Addr {
	return a.Conn.LocalAddr()
}

func (a *SessionAgentImpl) RemoteAddr() net.Addr {
	return a.Conn.RemoteAddr()
}

func (a *SessionAgentImpl) Close() {
	a.Conn.Close()
}

func (a *SessionAgentImpl) Destroy() {
	a.Conn.Destroy()
}

func (a *SessionAgentImpl) UserData() any {
	return a.Data
}

func (a *SessionAgentImpl) SetUserData(data any) {
	a.Data = data
}
