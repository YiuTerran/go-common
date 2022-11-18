package gate

import (
	"context"
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/base/structs/rpc"
	"github.com/YiuTerran/go-common/network"
	"github.com/YiuTerran/go-common/network/tcp"
)

// TcpGate 一个封装后的TCP服务
type TcpGate struct {
	//监听地址
	Addr string
	//最大连接数
	MaxConnNum int
	//消息处理器
	MsgProcessor network.MsgProcessor
	//对外通信
	RPCServer rpc.IServer
	//二进制分包
	BinaryParser tcp.IParser
}

func (gate *TcpGate) Processor() network.MsgProcessor {
	return gate.MsgProcessor
}

func (gate *TcpGate) AgentChanRPC() rpc.IServer {
	return gate.RPCServer
}

func (gate *TcpGate) Run(ctx context.Context) {
	if gate.Addr == "" {
		log.Fatal("tcp server addr not set")
		return
	}
	tcpServer := new(tcp.Server)
	tcpServer.Addr = gate.Addr
	tcpServer.MaxConnNum = gate.MaxConnNum
	tcpServer.Parser = gate.BinaryParser
	tcpServer.NewSessionFunc = func(conn *tcp.Conn) network.Session {
		a := &SessionAgentImpl{Conn: conn, Gate: gate}
		if gate.RPCServer != nil {
			gate.RPCServer.Go(AgentCreatedEvent, a)
		}
		return a
	}

	tcpServer.Start()
	<-ctx.Done()
	tcpServer.Close()
}

func (gate *TcpGate) OnDestroy() {}
