package gate

import (
	"github.com/YiuTerran/go-common/base/structs/rpc"
	"github.com/YiuTerran/go-common/network"
	"github.com/YiuTerran/go-common/network/tcp"
)

type TcpClient struct {
	Server        string
	MsgProcessor  network.MsgProcessor
	RPCServer     rpc.IServer
	BinaryParser  tcp.IParser
	AutoReconnect bool
	UserData      any
}

func (c *TcpClient) Processor() network.MsgProcessor {
	return c.MsgProcessor
}

func (c *TcpClient) AgentChanRPC() rpc.IServer {
	return c.RPCServer
}

func (c *TcpClient) Run(closeSig chan struct{}) {
	var tcpClient *tcp.Client
	if c.Server != "" {
		tcpClient = &tcp.Client{
			Addr:          c.Server,
			AutoReconnect: c.AutoReconnect,
			Parser:        c.BinaryParser,
			NewAgentFunc: func(conn *tcp.Conn) network.Session {
				a := &agent{conn: conn, gate: c}
				if c.RPCServer != nil {
					c.RPCServer.Go(AgentCreatedEvent, a, c.UserData)
				}
				return a
			},
		}
	}
	if tcpClient != nil {
		tcpClient.Start()
	}
	<-closeSig
	if tcpClient != nil {
		tcpClient.Close()
	}
}

func (c *TcpClient) OnDestroy() {}
