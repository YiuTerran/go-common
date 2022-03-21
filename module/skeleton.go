package module

import (
	"time"
)

type Skeleton struct {
	GoLen              int //回调缓冲区长度限制
	TimerDispatcherLen int //定时器缓冲区长度限制
	AsyncCallLen       int //异步调用结果缓冲区长度限制
	ChanRPCServer      *Server

	g             *CallbackChn
	dispatcher    *Dispatcher
	client        *Client
	server        *Server
	commandServer *Server
}

func (s *Skeleton) Init() {
	s.g = NewGo(s.GoLen)
	s.dispatcher = NewDispatcher(s.TimerDispatcherLen)
	s.client = NewClient(s.AsyncCallLen)
	s.server = s.ChanRPCServer

	if s.server == nil {
		s.server = NewServer(0)
	}
	s.commandServer = NewServer(0)
}

func (s *Skeleton) Run(closeSig chan struct{}) {
	for {
		select {
		case <-closeSig:
			s.commandServer.Close()
			s.server.Close()
			for !s.g.Idle() || !s.client.Idle() {
				s.g.Close()
				s.client.Close()
			}
			return
		case ri := <-s.client.ChanAsyncRet:
			s.client.Cb(ri)
		case ci := <-s.server.chanCall:
			s.server.Exec(ci)
		case ci := <-s.commandServer.chanCall:
			s.commandServer.Exec(ci)
		case cb := <-s.g.ChanCb:
			s.g.Cb(cb)
		case t := <-s.dispatcher.ChanTimer:
			t.Cb()
		}
	}
}

func (s *Skeleton) AfterFunc(d time.Duration, cb func()) *timer.Timer {
	if s.TimerDispatcherLen == 0 {
		panic("invalid TimerDispatcherLen")
	}

	return s.dispatcher.AfterFunc(d, cb)
}

func (s *Skeleton) CronFunc(cronExpr *timer.CronExpr, cb func()) *timer.Cron {
	if s.TimerDispatcherLen == 0 {
		panic("invalid TimerDispatcherLen")
	}

	return s.dispatcher.CronFunc(cronExpr, cb)
}

func (s *Skeleton) Go(f func(), cb func()) {
	if s.GoLen == 0 {
		panic("invalid GoLen")
	}

	s.g.Go(f, cb)
}

func (s *Skeleton) NewLinearContext() *LinearContext {
	if s.GoLen == 0 {
		panic("invalid GoLen")
	}

	return s.g.NewLinearContext()
}

func (s *Skeleton) AsyncCall(server *Server, id any, args ...any) {
	if s.AsyncCallLen == 0 {
		panic("invalid AsyncCallLen")
	}

	s.client.Attach(server)
	s.client.AsyncCall(id, args...)
}

func (s *Skeleton) RegisterChanRPC(id any, f any) {
	if s.ChanRPCServer == nil {
		panic("invalid ChanRPCServer")
	}
	s.server.Register(id, f)
}
