package module

import (
	"errors"
	"fmt"
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/base/structs/chanx"
)

const initBufferSize = 2048

// RpcServer one rpcServer per goroutine (goroutine not safe)
// one rpcClient per goroutine (goroutine not safe)
type RpcServer struct {
	// id -> function
	//
	// function:
	// func(args []any)
	// func(args []any) any
	// func(args []any) []any
	functions map[any]any
	ChanCall  *chanx.UnboundedChan[*callInfo]
}

type callInfo struct {
	f    any
	args []any
	//仅需往里面写入
	chanRet chan<- *retInfo
	cb      any
}

type retInfo struct {
	// nil
	// any
	// []any
	ret any
	err error
	// callback:
	// func(err error)
	// func(ret any, err error)
	// func(ret []any, err error)
	cb any
}

type RpcClient struct {
	s                *RpcServer
	chanSyncRet      chan *retInfo
	chanAsyncRet     *chanx.UnboundedChan[*retInfo]
	pendingAsyncCall int
}

func NewRpcServer() *RpcServer {
	s := new(RpcServer)
	s.functions = make(map[any]any)
	s.ChanCall = chanx.NewUnboundedChan[*callInfo](initBufferSize)
	return s
}

func assert(i any) []any {
	if i == nil {
		return nil
	} else {
		return i.([]any)
	}
}

// Register 注册命令回调.
// NOTE: 这些回调函数运行在同一个协程里，不要在回调函数中进行阻塞
// you must call the function before calling CreateClient and CallbackChn
func (s *RpcServer) Register(id any, f any) {
	switch f.(type) {
	case func([]any):
	case func([]any) any:
	case func([]any) []any:
	default:
		panic(fmt.Sprintf("function id %v: definition of function is invalid", id))
	}
	if _, ok := s.functions[id]; ok {
		panic(fmt.Sprintf("function id %v: already registered", id))
	}
	s.functions[id] = f
}

func (s *RpcServer) ret(ci *callInfo, ri *retInfo) (err error) {
	if ci.chanRet == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()

	ri.cb = ci.cb
	ci.chanRet <- ri
	return
}

func (s *RpcServer) exec(ci *callInfo) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.PanicStack("", r)
			_ = s.ret(ci, &retInfo{err: fmt.Errorf("%v", r)})
		}
	}()

	// execute
	switch ci.f.(type) {
	case func([]any):
		ci.f.(func([]any))(ci.args)
		return s.ret(ci, &retInfo{})
	case func([]any) any:
		ret := ci.f.(func([]any) any)(ci.args)
		return s.ret(ci, &retInfo{ret: ret})
	case func([]any) []any:
		ret := ci.f.(func([]any) []any)(ci.args)
		return s.ret(ci, &retInfo{ret: ret})
	}
	panic("bug for invalid func type")
}

func (s *RpcServer) execIgnoreError(ci *callInfo) {
	err := s.exec(ci)
	if err != nil {
		log.Error("callback error: %v", err)
	}
}

// Go 在Server模块主线程里面运行命令，异步执行，goroutine safe
func (s *RpcServer) Go(id any, args ...any) {
	f := s.functions[id]
	if f == nil {
		return
	}

	defer func() {
		recover()
	}()

	s.ChanCall.In <- &callInfo{
		f:    f,
		args: args,
	}
}

// Call0 同步调用，无返回结果，goroutine safe
func (s *RpcServer) Call0(id any, args ...any) error {
	return s.CreateClient(true).Call0(id, args...)
}

// Call1 同步调用，单个返回结果，goroutine safe
func (s *RpcServer) Call1(id any, args ...any) (any, error) {
	return s.CreateClient(true).Call1(id, args...)
}

// CallN 同步调用，返回数组，goroutine safe
func (s *RpcServer) CallN(id any, args ...any) ([]any, error) {
	return s.CreateClient(true).CallN(id, args...)
}

func (s *RpcServer) Close() {
	s.ChanCall.Close()
	for ci := range s.ChanCall.Out {
		_ = s.ret(ci, &retInfo{
			err: errors.New("chanrpc rpcServer closed"),
		})
	}
}

// CreateClient 打开rpc，goroutine safe
func (s *RpcServer) CreateClient(noAsync bool) *RpcClient {
	c := NewRpcClient(noAsync)
	c.Attach(s)
	return c
}

func NewRpcClient(noAsync bool) *RpcClient {
	c := new(RpcClient)
	c.chanSyncRet = make(chan *retInfo, 1)
	size := initBufferSize
	if noAsync {
		size = 0
	}
	c.chanAsyncRet = chanx.NewUnboundedChan[*retInfo](size)
	return c
}

func (c *RpcClient) Attach(s *RpcServer) {
	c.s = s
}

func (c *RpcClient) call(ci *callInfo) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()
	//阻塞
	c.s.ChanCall.In <- ci
	return
}

func (c *RpcClient) f(id any, n int) (f any, err error) {
	if c.s == nil {
		err = errors.New("rpcServer not attached")
		return
	}

	f = c.s.functions[id]
	if f == nil {
		err = fmt.Errorf("function id %v: function not registered", id)
		return
	}

	var ok bool
	switch n {
	case 0:
		_, ok = f.(func([]any))
	case 1:
		_, ok = f.(func([]any) any)
	case 2:
		_, ok = f.(func([]any) []any)
	default:
		panic("bug")
	}

	if !ok {
		err = fmt.Errorf("function id %v: return type mismatch", id)
	}
	return
}

func (c *RpcClient) Call0(id any, args ...any) error {
	f, err := c.f(id, 0)
	if err != nil {
		return err
	}

	err = c.call(&callInfo{
		f:       f,
		args:    args,
		chanRet: c.chanSyncRet,
	})
	if err != nil {
		return err
	}

	ri := <-c.chanSyncRet
	return ri.err
}

func (c *RpcClient) Call1(id any, args ...any) (any, error) {
	f, err := c.f(id, 1)
	if err != nil {
		return nil, err
	}

	err = c.call(&callInfo{
		f:       f,
		args:    args,
		chanRet: c.chanSyncRet,
	})
	if err != nil {
		return nil, err
	}

	ri := <-c.chanSyncRet
	return ri.ret, ri.err
}

func (c *RpcClient) CallN(id any, args ...any) ([]any, error) {
	f, err := c.f(id, 2)
	if err != nil {
		return nil, err
	}

	err = c.call(&callInfo{
		f:       f,
		args:    args,
		chanRet: c.chanSyncRet,
	})
	if err != nil {
		return nil, err
	}

	ri := <-c.chanSyncRet
	return assert(ri.ret), ri.err
}
func (c *RpcClient) asyncCall(id any, args []any, cb any, n int) {
	f, err := c.f(id, n)
	if err != nil {
		c.chanAsyncRet.In <- &retInfo{err: err, cb: cb}
		return
	}

	err = c.call(&callInfo{
		f:       f,
		args:    args,
		chanRet: c.chanAsyncRet.In,
		cb:      cb,
	})
	if err != nil {
		c.chanAsyncRet.In <- &retInfo{err: err, cb: cb}
		return
	}
}

func (c *RpcClient) AsyncCall(id any, _args ...any) {
	if len(_args) < 1 {
		panic("callback function not found")
	}

	args := _args[:len(_args)-1]
	cb := _args[len(_args)-1]

	var n int
	switch cb.(type) {
	case func(error):
		n = 0
	case func(any, error):
		n = 1
	case func([]any, error):
		n = 2
	default:
		panic("definition of callback function is invalid")
	}
	c.asyncCall(id, args, cb, n)
	c.pendingAsyncCall++
}

func execCb(ri *retInfo) {
	defer func() {
		if r := recover(); r != nil {
			log.PanicStack("", r)
		}
	}()

	// execute
	switch ri.cb.(type) {
	case func(error):
		ri.cb.(func(error))(ri.err)
	case func(any, error):
		ri.cb.(func(any, error))(ri.ret, ri.err)
	case func([]any, error):
		ri.cb.(func([]any, error))(assert(ri.ret), ri.err)
	default:
		panic("bug")
	}
	return
}

func (c *RpcClient) cb(ri *retInfo) {
	c.pendingAsyncCall--
	execCb(ri)
}

func (c *RpcClient) Close() {
	for c.pendingAsyncCall > 0 {
		c.cb(<-c.chanAsyncRet.Out)
	}
}

func (c *RpcClient) Idle() bool {
	return c.pendingAsyncCall == 0
}
