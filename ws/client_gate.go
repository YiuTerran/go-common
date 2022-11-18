package ws

import (
	"context"
	"github.com/YiuTerran/go-common/base/structs/rpc"
	"github.com/YiuTerran/go-common/network"
	"github.com/YiuTerran/go-common/network/gate"
	"time"
)

/**
  *  @author tryao
  *  @date 2022/03/22 11:32
**/

// ClientGate 客户端的包装，可以用来实现Module
type ClientGate struct {
	Server        string
	MsgTextFormat bool
	HttpTimeout   time.Duration
	MsgProcessor  network.MsgProcessor
	RPCServer     rpc.IServer
	AutoReconnect bool
	UserData      any
}

func (cg *ClientGate) Processor() network.MsgProcessor {
	return cg.MsgProcessor
}

func (cg *ClientGate) AgentChanRPC() rpc.IServer {
	return cg.RPCServer
}

func (cg *ClientGate) Run(ctx context.Context) {
	var wsClient *Client
	if cg.Server != "" {
		wsClient = &Client{
			Addr:             cg.Server,
			ConnNum:          1,
			ConnectInterval:  3 * time.Second,
			HandshakeTimeout: cg.HttpTimeout,
			AutoReconnect:    cg.AutoReconnect,
			TextFormat:       cg.MsgTextFormat,
			NewSessionFunc: func(conn *Conn) network.Session {
				a := &gate.SessionAgentImpl{Conn: conn, Gate: cg}
				if cg.RPCServer != nil {
					cg.RPCServer.Go(gate.AgentCreatedEvent, a, cg.UserData)
				}
				return a
			},
		}
	}
	if wsClient != nil {
		wsClient.Start()
	}
	<-ctx.Done()
	if wsClient != nil {
		wsClient.Close()
	}
}

func (cg *ClientGate) OnDestroy() {}
