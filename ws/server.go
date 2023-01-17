package ws

import (
	"crypto/tls"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/base/structs/set"
	"github.com/YiuTerran/go-common/network"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

/**
  *  @author tryao
  *  @date 2022/03/22 11:32
**/

// Server 是websocket服务端
type Server struct {
	Addr        string
	MaxMsgLen   uint32
	HTTPTimeout time.Duration
	//证书路径
	CertFile string
	//密钥路径
	KeyFile        string
	NewSessionFunc func(*Conn) network.Session
	AuthFunc       func(*http.Request) (bool, any)
	TextFormat     bool //纯文本还是二进制

	ln      net.Listener
	handler *handlerDTO
}

type handlerDTO struct {
	textFormat     bool
	authFunc       func(*http.Request) (bool, any)
	maxMsgLen      uint32
	newSessionFunc func(*Conn) network.Session
	upgrader       websocket.Upgrader
	conns          *set.Set[*websocket.Conn]
	mutexConns     sync.Mutex
	wg             sync.WaitGroup
}

type Option func(*Server)

func NewServer(port int, newSessionFunc func(*Conn) network.Session, options ...Option) *Server {
	if port <= 0 || port > 65535 {
		return nil
	}
	addr := fmt.Sprintf("0.0.0.0:%d", port)
	server := &Server{
		Addr:           addr,
		MaxMsgLen:      1024000,
		HTTPTimeout:    10 * time.Second,
		NewSessionFunc: newSessionFunc,
		TextFormat:     false,
	}
	for _, option := range options {
		option(server)
	}
	return server
}

func WithMaxMsgLen(num uint32) Option {
	return func(server *Server) {
		server.MaxMsgLen = num
	}
}

func WithHttpTimeout(duration time.Duration) Option {
	return func(server *Server) {
		server.HTTPTimeout = duration
	}
}

func WithHttpsCert(cert, key string) Option {
	return func(server *Server) {
		server.CertFile = cert
		server.KeyFile = key
	}
}

func WithAuthFunc(authFunc func(*http.Request) (bool, any)) Option {
	return func(server *Server) {
		server.AuthFunc = authFunc
	}
}

func WithTextFormat(usingText bool) Option {
	return func(server *Server) {
		server.TextFormat = usingText
	}
}

func getRealIP(req *http.Request) net.Addr {
	ip := req.Header.Get("X-FORWARDED-FOR")
	if ip == "" {
		ip = req.Header.Get("X-REAL-IP")
	}
	if ip != "" {
		ip = strings.Split(ip, ",")[0]
	} else {
		ip, _, _ = net.SplitHostPort(req.RemoteAddr)
	}
	q := net.ParseIP(ip)
	addr := &net.IPAddr{IP: q}
	return addr
}

func (handler *handlerDTO) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var (
		ok       bool
		userData any
	)
	if handler.authFunc != nil {
		if ok, userData = handler.authFunc(r); !ok {
			http.Error(w, "Forbidden", 403)
			return
		}
	}
	conn, err := handler.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Debug("upgrade error: %v", err)
		return
	}
	conn.SetReadLimit(int64(handler.maxMsgLen))

	handler.wg.Add(1)
	defer handler.wg.Done()

	handler.mutexConns.Lock()
	if handler.conns == nil {
		handler.mutexConns.Unlock()
		_ = conn.Close()
		return
	}
	handler.conns.AddItem(conn)
	handler.mutexConns.Unlock()

	wsConn := newWSConn(conn, handler.maxMsgLen, handler.textFormat)
	wsConn.remoteOriginIP = getRealIP(r)
	wsConn.userData = userData
	session := handler.newSessionFunc(wsConn)
	session.Run()

	// cleanup
	wsConn.Close()
	handler.mutexConns.Lock()
	handler.conns.RemoveItem(conn)
	handler.mutexConns.Unlock()
	session.OnClose()
}

func (server *Server) Start() {
	ln, err := net.Listen("tcp", server.Addr)
	if err != nil {
		log.Fatal("fail to start tcp server: %v", err)
	}

	if server.MaxMsgLen <= 0 {
		server.MaxMsgLen = 1024000
		log.Info("invalid MaxMsgLen, reset to %v", server.MaxMsgLen)
	}
	if server.HTTPTimeout <= 0 {
		server.HTTPTimeout = 10 * time.Second
		log.Info("invalid HTTPTimeout, reset to %v", server.HTTPTimeout)
	}
	if server.NewSessionFunc == nil {
		log.Fatal("NewSessionFunc must not be nil")
	}

	if server.CertFile != "" || server.KeyFile != "" {
		config := &tls.Config{}
		config.NextProtos = []string{"http/1.1"}

		var err error
		config.Certificates = make([]tls.Certificate, 1)
		config.Certificates[0], err = tls.LoadX509KeyPair(server.CertFile, server.KeyFile)
		if err != nil {
			log.Fatal("%v", err)
		}

		ln = tls.NewListener(ln, config)
	}

	server.ln = ln
	server.handler = &handlerDTO{
		textFormat:     server.TextFormat,
		authFunc:       server.AuthFunc,
		maxMsgLen:      server.MaxMsgLen,
		newSessionFunc: server.NewSessionFunc,
		conns:          set.NewSet[*websocket.Conn](),
		upgrader: websocket.Upgrader{
			HandshakeTimeout: server.HTTPTimeout,
			CheckOrigin:      func(_ *http.Request) bool { return true },
		},
	}

	httpServer := &http.Server{
		Addr:           server.Addr,
		Handler:        server.handler,
		ReadTimeout:    server.HTTPTimeout,
		WriteTimeout:   server.HTTPTimeout,
		MaxHeaderBytes: 1024,
	}

	go func() {
		if err = httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Fatal("fail to start websocket server:%v", err)
		}
	}()
}

func (server *Server) Close() {
	_ = server.ln.Close()

	server.handler.mutexConns.Lock()
	server.handler.conns.ForEach(func(conn *websocket.Conn) {
		_ = conn.Close()
	})
	server.handler.conns = nil
	server.handler.mutexConns.Unlock()

	server.handler.wg.Wait()
}
