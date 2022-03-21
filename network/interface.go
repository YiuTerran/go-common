package network

import "net"

// Session 每个连接在独立的协程里处理消息
type Session interface {
	// Run 阻塞通信循环
	Run()
	// OnClose 关闭连接回调
	OnClose()
}

// Conn 对于网络连接的抽象
type Conn interface {
	ReadMsg() ([]byte, error)
	WriteMsg(args ...[]byte) error
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
	Close()
	Destroy()
}

// MsgProcessor 是消息处理器
type MsgProcessor interface {
	// Route 路由消息 must goroutine safe
	Route(msg any, userData any) error
	// Unmarshal 反序列化消息，must goroutine safe
	Unmarshal(data []byte) (any, error)
	// Marshal 序列化消息 must goroutine safe
	Marshal(msg any) ([][]byte, error)
}
