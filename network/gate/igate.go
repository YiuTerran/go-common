package gate

import (
	"github.com/YiuTerran/go-common/base/structs/rpc"
	"github.com/YiuTerran/go-common/network"
)

const (
	AgentCreatedEvent     = "NewSessionFunc"
	AgentBeforeCloseEvent = "CloseAgent"
)

// IGate 路由
type IGate interface {
	Processor() network.MsgProcessor
	AgentChanRPC() rpc.IServer
}
