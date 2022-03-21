package module

import (
	"time"
)

// GoroutineMixIn 是一个封装后的独立协程
//在原本协程的基础上增加了协程间通信等功能
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

func (s *GoroutineMixIn) Run(closeSig chan struct{}) {
	for {
		select {
		case <-closeSig:
			s.RpcServer.Close()
			for !s.g.Idle() || !s.rpcClient.Idle() {
				s.g.Close()
				s.rpcClient.Close()
			}
			return
		case ri := <-s.rpcClient.chanAsyncRet.Out:
			s.rpcClient.cb(ri)
		case ci := <-s.RpcServer.ChanCall.Out:
			s.RpcServer.execIgnoreError(ci)
		case cb := <-s.g.ChanCb.Out:
			s.g.Cb(cb)
		case t := <-s.dispatcher.ChanTimer.Out:
			t.Cb()
		}
	}
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
