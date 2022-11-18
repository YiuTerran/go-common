package mock

import (
	"io"
	"net"
)

/**
  *  @author tryao
  *  @date 2022/08/03 14:10
**/

type Listener struct {
	network string
	addr    net.Addr
	ch      chan net.Conn
}

func ListenTCP(network string, addr *net.TCPAddr) (net.Listener, error) {
	return &Listener{
		network: network,
		addr:    addr,
		ch:      make(chan net.Conn, 1),
	}, nil
}

// Accept never return
func (l Listener) Accept() (net.Conn, error) {
	return <-l.ch, io.EOF
}

func (l Listener) Close() error {
	defer func() {
		if i := recover(); i != nil {
			//just ignore
		}
	}()
	close(l.ch)
	return nil
}

func (l Listener) Addr() net.Addr {
	return l.addr
}
