package gate

import (
	"context"
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/base/structs/rpc"
	"github.com/YiuTerran/go-common/network"
	"github.com/YiuTerran/go-common/network/udp"
)

/**  由于UDP没有连接，所以client就不需要单独占一个协程了，可以类似HTTP那样每次请求一个独立协程
  *  实际上UDP通信，除非是简单的单向通知，一般需要同时打开服务端和客户端
  *  @author tryao
  *  @date 2022/03/24 10:43
**/

type UdpGate struct {
	//监听地址
	Addr string
	//消息处理器
	MsgProcessor network.MsgProcessor
	//对外通信
	RPCServer rpc.IServer
	//失败重试次数，默认0
	FailTry int
}

func (u *UdpGate) Processor() network.MsgProcessor {
	return u.MsgProcessor
}

func (u *UdpGate) AgentChanRPC() rpc.IServer {
	return u.RPCServer
}

func (u *UdpGate) Run(ctx context.Context) {
	if u.Addr == "" {
		log.Fatal("udp server listen addr not set")
		return
	}
	server := &udp.Server{
		Addr:      u.Addr,
		Processor: u.MsgProcessor,
		FailTry:   u.FailTry,
	}
	server.Start()
	<-ctx.Done()
	server.Close()
}

func (u *UdpGate) OnDestroy() {

}
