package ws

/**
  *  @author tryao
  *  @date 2022/03/22 11:33
**/
import (
	"errors"
	"github.com/YiuTerran/go-common/base/structs/chanx"
	"net"
	"sync"

	"github.com/gorilla/websocket"
)

const (
	initBufferSize = 2048
)

type Conn struct {
	sync.Mutex
	conn           *websocket.Conn
	writeChan      *chanx.UnboundedChan[[]byte]
	maxMsgLen      uint32
	closeFlag      bool
	remoteOriginIP net.Addr
	userData       any
}

func (wsConn *Conn) UserData() any {
	return wsConn.userData
}

func newWSConn(conn *websocket.Conn, maxMsgLen uint32, textFormat bool) *Conn {
	wsConn := new(Conn)
	wsConn.conn = conn
	wsConn.writeChan = chanx.NewUnboundedChan[[]byte](initBufferSize)
	wsConn.maxMsgLen = maxMsgLen
	msgType := websocket.BinaryMessage
	if textFormat {
		msgType = websocket.TextMessage
	}
	go func() {
		for b := range wsConn.writeChan.Out {
			if b == nil {
				break
			}
			err := conn.WriteMessage(msgType, b)
			if err != nil {
				break
			}
		}

		_ = conn.Close()
		wsConn.Lock()
		wsConn.closeFlag = true
		wsConn.Unlock()
	}()

	return wsConn
}

func (wsConn *Conn) doDestroy() {
	_ = wsConn.conn.UnderlyingConn().(*net.TCPConn).SetLinger(0)
	_ = wsConn.conn.Close()

	if !wsConn.closeFlag {
		wsConn.writeChan.Close()
		wsConn.closeFlag = true
	}
}

func (wsConn *Conn) Destroy() {
	wsConn.Lock()
	defer wsConn.Unlock()

	wsConn.doDestroy()
}

func (wsConn *Conn) Close() {
	wsConn.Lock()
	defer wsConn.Unlock()
	if wsConn.closeFlag {
		return
	}

	wsConn.doWrite(nil)
	wsConn.closeFlag = true
}

func (wsConn *Conn) doWrite(b []byte) {
	wsConn.writeChan.In <- b
}

func (wsConn *Conn) LocalAddr() net.Addr {
	return wsConn.conn.LocalAddr()
}

func (wsConn *Conn) RemoteAddr() net.Addr {
	if wsConn.remoteOriginIP != nil {
		return wsConn.remoteOriginIP
	}
	return wsConn.conn.RemoteAddr()
}

// ReadMsg goroutine not safe
func (wsConn *Conn) ReadMsg() ([]byte, error) {
	_, b, err := wsConn.conn.ReadMessage()
	return b, err
}

// WriteMsg args must not be modified by the others goroutines
func (wsConn *Conn) WriteMsg(args ...[]byte) error {
	wsConn.Lock()
	defer wsConn.Unlock()
	if wsConn.closeFlag {
		return nil
	}

	// get len
	var msgLen uint32
	for i := 0; i < len(args); i++ {
		msgLen += uint32(len(args[i]))
	}

	// check len
	if msgLen > wsConn.maxMsgLen {
		return errors.New("message too long")
	} else if msgLen < 1 {
		return errors.New("message too short")
	}

	// don't copy
	if len(args) == 1 {
		wsConn.doWrite(args[0])
		return nil
	}

	// merge the args
	msg := make([]byte, msgLen)
	l := 0
	for i := 0; i < len(args); i++ {
		copy(msg[l:], args[i])
		l += len(args[i])
	}

	wsConn.doWrite(msg)

	return nil
}
