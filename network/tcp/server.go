package tcp

import (
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/base/structs/set"
	"github.com/YiuTerran/go-common/network"
	"net"
	"sync"
	"time"
)

type Server struct {
	Addr           string
	MaxConnNum     int
	NewSessionFunc func(*Conn) network.Session

	ln        net.Listener
	cons      *set.Set[net.Conn]
	mutexCons sync.Mutex
	wgLn      sync.WaitGroup
	wgCons    sync.WaitGroup

	// msg parser
	Parser IParser
}

func (server *Server) Start() {
	server.init()
	go server.run()
}

func (server *Server) init() {
	ln, err := net.Listen("tcp", server.Addr)
	if err != nil {
		log.Fatal("fail to start tcp server:%v", err)
	}
	if server.NewSessionFunc == nil {
		log.Fatal("NewSessionFunc must not be nil")
	}

	server.ln = ln
	server.cons = set.NewSet[net.Conn]()

	// msg parser
	if server.Parser == nil {
		server.Parser = NewDefaultParser()
	}
}

func (server *Server) run() {
	server.wgLn.Add(1)
	defer server.wgLn.Done()

	var tempDelay time.Duration
	for {
		conn, err := server.ln.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				log.Info("accept error: %v; retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			return
		}
		tempDelay = 0

		server.mutexCons.Lock()
		if server.MaxConnNum > 0 && server.cons.Size() >= server.MaxConnNum {
			server.mutexCons.Unlock()
			_ = conn.Close()
			log.Warn("too many tcp connections")
			continue
		}
		server.cons.AddItem(conn)
		server.mutexCons.Unlock()

		server.wgCons.Add(1)

		tcpConn := newConn(conn, server.Parser)
		session := server.NewSessionFunc(tcpConn)
		go func() {
			session.Run()

			// cleanup
			tcpConn.Close()
			server.mutexCons.Lock()
			server.cons.RemoveItem(conn)
			server.mutexCons.Unlock()
			session.OnClose()

			server.wgCons.Done()
		}()
	}
}

func (server *Server) Close() {
	_ = server.ln.Close()
	server.wgLn.Wait()

	server.mutexCons.Lock()
	server.cons.ForEach(func(conn net.Conn) {
		_ = conn.Close()
	})
	server.cons = nil
	server.mutexCons.Unlock()
	server.wgCons.Wait()
}
