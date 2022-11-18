package tcp

import (
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/base/structs/chanx"
	"net"
	"sync"
)

type Conn struct {
	sync.Mutex
	conn      net.Conn
	writeChan *chanx.UnboundedChan[[]byte]
	closeFlag bool
	parser    IParser
}

func newConn(conn net.Conn, parser IParser) *Conn {
	tcpConn := new(Conn)
	tcpConn.conn = conn
	tcpConn.writeChan = chanx.NewUnboundedChan[[]byte](100)
	tcpConn.parser = parser

	go func() {
		for b := range tcpConn.writeChan.Out {
			if b == nil {
				break
			}
			_, err := conn.Write(b)
			if err != nil {
				log.Error("fail to write tcp chan:%+v", err)
				break
			}
		}

		_ = conn.Close()
		tcpConn.Lock()
		tcpConn.closeFlag = true
		tcpConn.Unlock()
	}()

	return tcpConn
}

func (c *Conn) doDestroy() {
	_ = c.conn.(*net.TCPConn).SetLinger(0)
	_ = c.conn.Close()

	if !c.closeFlag {
		c.writeChan.Close()
		c.closeFlag = true
	}
}

func (c *Conn) Destroy() {
	c.Lock()
	defer c.Unlock()

	c.doDestroy()
}

func (c *Conn) Close() {
	c.Lock()
	defer c.Unlock()
	if c.closeFlag {
		return
	}

	c.doWrite(nil)
	c.closeFlag = true
}

func (c *Conn) doWrite(b []byte) {
	c.writeChan.In <- b
}

// b must not be modified by the others goroutines
func (c *Conn) Write(b []byte) {
	c.Lock()
	defer c.Unlock()
	if c.closeFlag || b == nil {
		return
	}

	c.doWrite(b)
}

func (c *Conn) Read(b []byte) (int, error) {
	return c.conn.Read(b)
}

func (c *Conn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *Conn) ReadMsg() ([]byte, error) {
	return c.parser.Read(c)
}

func (c *Conn) WriteMsg(args ...[]byte) error {
	return c.parser.Write(c, args...)
}
