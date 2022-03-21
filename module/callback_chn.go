package module

import (
	"container/list"
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/base/structs/chanx"
	"sync"
)

//CallbackChn 是回调函数的队列包装
type CallbackChn struct {
	ChanCb    *chanx.UnboundedChan[func()]
	pendingGo int
}

// LinearCallbackChn 包括一个待执行函数和执行完毕之后回调的函数
type LinearCallbackChn struct {
	f  func()
	cb func()
}

type LinearContext struct {
	g              *CallbackChn
	linearGo       *list.List
	mutexLinearGo  sync.Mutex
	mutexExecution sync.Mutex
}

func NewCallbackChn() *CallbackChn {
	g := new(CallbackChn)
	g.ChanCb = chanx.NewUnboundedChan[func()](initBufferSize)
	return g
}

func (g *CallbackChn) Go(f func(), cb func()) {
	g.pendingGo++

	go func() {
		defer func() {
			g.ChanCb.In <- cb
			if r := recover(); r != nil {
				log.PanicStack("", r)
			}
		}()
		f()
	}()
}

func (g *CallbackChn) Cb(cb func()) {
	defer func() {
		g.pendingGo--
		if r := recover(); r != nil {
			log.PanicStack("", r)
		}
	}()

	if cb != nil {
		cb()
	}
}

// Close 关闭之前需要执行玩所有回调
func (g *CallbackChn) Close() {
	for g.pendingGo > 0 {
		g.Cb(<-g.ChanCb.Out)
	}
}

func (g *CallbackChn) Idle() bool {
	return g.pendingGo == 0
}

func (g *CallbackChn) NewLinearContext() *LinearContext {
	c := new(LinearContext)
	c.g = g
	c.linearGo = list.New()
	return c
}

func (c *LinearContext) Go(f func(), cb func()) {
	c.g.pendingGo++

	c.mutexLinearGo.Lock()
	c.linearGo.PushBack(&LinearCallbackChn{f: f, cb: cb})
	c.mutexLinearGo.Unlock()

	go func() {
		c.mutexExecution.Lock()
		defer c.mutexExecution.Unlock()

		c.mutexLinearGo.Lock()
		e := c.linearGo.Remove(c.linearGo.Front()).(*LinearCallbackChn)
		c.mutexLinearGo.Unlock()

		defer func() {
			c.g.ChanCb.In <- e.cb
			if r := recover(); r != nil {
				log.PanicStack("", r)
			}
		}()

		e.f()
	}()
}
