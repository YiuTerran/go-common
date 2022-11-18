package transport

import (
	"fmt"
	"github.com/YiuTerran/go-common/base/log"

	"net"
	"strings"
	"sync"
	"time"
)

var (
	bufferSize uint16 = 65535 - 20 - 8 // IPv4 max size - IPv4 Header size - UDP Header size
)

// Connection wrapper around net.Conn.
type Connection interface {
	net.Conn

	Key() ConnectionKey
	Network() string
	Streamed() bool
	String() string
	ReadFrom(buf []byte) (num int, raddr net.Addr, err error)
	WriteTo(buf []byte, raddr net.Addr) (num int, err error)
	Fields() log.Fields
}

// Connection implementation.
type connection struct {
	baseConn net.Conn
	key      ConnectionKey
	network  string
	laddr    net.Addr
	raddr    net.Addr
	streamed bool
	mu       sync.RWMutex

	fields log.Fields
}

func NewConnection(baseConn net.Conn, key ConnectionKey, network string, fields log.Fields) Connection {
	var stream bool
	switch baseConn.(type) {
	case net.PacketConn:
		stream = false
	default:
		stream = true
	}

	conn := &connection{
		baseConn: baseConn,
		key:      key,
		network:  network,
		laddr:    baseConn.LocalAddr(),
		raddr:    baseConn.RemoteAddr(),
		streamed: stream,
	}
	conn.fields = fields.
		WithPrefix("transport.Connection").
		WithFields(log.Fields{
			"connection_ptr": fmt.Sprintf("%p", conn),
			"connection_key": conn.Key(),
		})

	return conn
}

func (conn *connection) String() string {
	if conn == nil {
		return "<nil>"
	}

	fields := conn.Fields().WithFields(log.Fields{
		"key":         conn.Key(),
		"network":     conn.Network(),
		"local_addr":  conn.LocalAddr(),
		"remote_addr": conn.RemoteAddr(),
	})

	return fmt.Sprintf("transport.Connection<%s>", fields)
}

func (conn *connection) Fields() log.Fields {
	return conn.fields
}

func (conn *connection) Key() ConnectionKey {
	return conn.key
}

func (conn *connection) Streamed() bool {
	return conn.streamed
}

func (conn *connection) Network() string {
	return strings.ToUpper(conn.network)
}

func (conn *connection) Read(buf []byte) (int, error) {
	var (
		num int
		err error
	)

	num, err = conn.baseConn.Read(buf)

	if err != nil {
		return num, &ConnectionError{
			err,
			"read",
			conn.Network(),
			fmt.Sprintf("%v", conn.RemoteAddr()),
			fmt.Sprintf("%v", conn.LocalAddr()),
			fmt.Sprintf("%p", conn),
		}
	}

	return num, err
}

func (conn *connection) ReadFrom(buf []byte) (num int, raddr net.Addr, err error) {
	num, raddr, err = conn.baseConn.(net.PacketConn).ReadFrom(buf)
	if err != nil {
		return num, raddr, &ConnectionError{
			err,
			"read",
			conn.Network(),
			fmt.Sprintf("%v", raddr),
			fmt.Sprintf("%v", conn.LocalAddr()),
			fmt.Sprintf("%p", conn),
		}
	}

	return num, raddr, err
}

func (conn *connection) Write(buf []byte) (int, error) {
	var (
		num int
		err error
	)

	num, err = conn.baseConn.Write(buf)
	if err != nil {
		return num, &ConnectionError{
			err,
			"write",
			conn.Network(),
			fmt.Sprintf("%v", conn.LocalAddr()),
			fmt.Sprintf("%v", conn.RemoteAddr()),
			fmt.Sprintf("%p", conn),
		}
	}

	return num, err
}

func (conn *connection) WriteTo(buf []byte, raddr net.Addr) (num int, err error) {
	num, err = conn.baseConn.(net.PacketConn).WriteTo(buf, raddr)
	if err != nil {
		return num, &ConnectionError{
			err,
			"write",
			conn.Network(),
			fmt.Sprintf("%v", conn.LocalAddr()),
			fmt.Sprintf("%v", raddr),
			fmt.Sprintf("%p", conn),
		}
	}

	return num, err
}

func (conn *connection) LocalAddr() net.Addr {
	return conn.baseConn.LocalAddr()
}

func (conn *connection) RemoteAddr() net.Addr {
	return conn.baseConn.RemoteAddr()
}

func (conn *connection) Close() error {
	err := conn.baseConn.Close()
	if err != nil {
		return &ConnectionError{
			err,
			"close",
			conn.Network(),
			"",
			"",
			fmt.Sprintf("%p", conn),
		}
	}
	return nil
}

func (conn *connection) SetDeadline(t time.Time) error {
	return conn.baseConn.SetDeadline(t)
}

func (conn *connection) SetReadDeadline(t time.Time) error {
	return conn.baseConn.SetReadDeadline(t)
}

func (conn *connection) SetWriteDeadline(t time.Time) error {
	return conn.baseConn.SetWriteDeadline(t)
}
