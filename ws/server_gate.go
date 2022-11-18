package ws

import (
	"context"
	"github.com/YiuTerran/go-common/base/structs/rpc"
	"github.com/YiuTerran/go-common/network"
	"github.com/YiuTerran/go-common/network/gate"
	"net/http"
	"time"
)

/**
  *  @author tryao
  *  @date 2022/03/22 14:27
**/

// ServerGate websocket的服务端封装，用来实现Module
type ServerGate struct {
	MaxMsgLen     uint32
	MsgProcessor  network.MsgProcessor
	MsgTextFormat bool
	AuthFunc      func(*http.Request) (bool, any)
	RPCServer     rpc.IServer

	Addr        string
	HTTPTimeout time.Duration
	CertFile    string
	KeyFile     string
}

func (sg *ServerGate) Processor() network.MsgProcessor {
	return sg.MsgProcessor
}

func (sg *ServerGate) AgentChanRPC() rpc.IServer {
	return sg.RPCServer
}

func (sg *ServerGate) Run(ctx context.Context) {
	var wsServer *Server
	if sg.Addr != "" {
		wsServer = new(Server)
		wsServer.Addr = sg.Addr
		wsServer.TextFormat = sg.MsgTextFormat
		wsServer.AuthFunc = sg.AuthFunc
		wsServer.MaxMsgLen = sg.MaxMsgLen
		wsServer.HTTPTimeout = sg.HTTPTimeout
		wsServer.CertFile = sg.CertFile
		wsServer.KeyFile = sg.KeyFile
		wsServer.NewSessionFunc = func(conn *Conn) network.Session {
			a := &gate.SessionAgentImpl{Conn: conn, Gate: sg, Data: conn.UserData()}
			if sg.RPCServer != nil {
				sg.RPCServer.Go(gate.AgentCreatedEvent, a)
			}
			return a
		}
	}
	if wsServer != nil {
		wsServer.Start()
	}
	<-ctx.Done()
	if wsServer != nil {
		wsServer.Close()
	}
}

func (sg *ServerGate) OnDestroy() {}
