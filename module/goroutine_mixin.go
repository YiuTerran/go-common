package module

import (
	"context"
	"github.com/YiuTerran/go-common/base/structs/rpc"
	"time"
)

// GoroutineMixIn 是一个封装后的独立协程
// 在原本协程的基础上增加了协程间通信等功能
type GoroutineMixIn struct {
	*RpcServer

	g          *CallbackChn
	dispatcher *Dispatcher
	rpcClient  *RpcClient
}

func NewGoroutineMixIn() *GoroutineMixIn {
	var s GoroutineMixIn
	s.g = NewCallbackChn()
	s.dispatcher = NewDispatcher()
	s.rpcClient = NewRpcClient(false)
	s.RpcServer = NewRpcServer()
	return &s
}

func (s *GoroutineMixIn) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			s.RpcServer.Close()
			if !s.g.Idle() || !s.rpcClient.Idle() {
				s.g.Close()
				s.rpcClient.Close()
			}
			return
		case ri := <-s.rpcClient.chanAsyncRet.Out:
			s.rpcClient.cb(ri)
		case cb := <-s.g.ChanCb.Out:
			s.g.Cb(cb)
		case ci := <-s.RpcServer.ChanCall.Out:
			s.RpcServer.execIgnoreError(ci)
		case t := <-s.dispatcher.ChanTimer.Out:
			t.Cb()
		}
	}
}

func (s *GoroutineMixIn) PendingCallSize() int {
	return s.g.pendingGo + s.rpcClient.pendingAsyncCall
}

func (s *GoroutineMixIn) DispatcherSize() int {
	return s.dispatcher.ChanTimer.Len()
}

func (s *GoroutineMixIn) ChanCallSize() int {
	return s.ChanCall.Len()
}

func (s *GoroutineMixIn) AfterFunc(d time.Duration, cb func()) *Timer {
	return s.dispatcher.AfterFunc(d, cb)
}

func (s *GoroutineMixIn) CronFunc(cronExpr *CronExpr, cb func()) *Cron {
	return s.dispatcher.CronFunc(cronExpr, cb)
}

func (s *GoroutineMixIn) Go(f func(), cb func()) {
	s.g.Go(f, cb)
}

func (s *GoroutineMixIn) NewLinearContext() *LinearContext {
	return s.g.NewLinearContext()
}

func (s *GoroutineMixIn) AsyncCall(server *RpcServer, id any, args ...any) {
	s.rpcClient.Attach(server)
	s.rpcClient.AsyncCall(id, args...)
}

func (s *GoroutineMixIn) RPC() rpc.IServer {
	return s.RpcServer
}

// Tags 这里相当于写了个Module接口的默认实现，因为tags一般可以没有
func (s *GoroutineMixIn) Tags() []string {
	return nil
}

// OnDestroy 默认啥也不用做
func (s *GoroutineMixIn) OnDestroy() {

}
