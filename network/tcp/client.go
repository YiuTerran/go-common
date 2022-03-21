package tcp

import (
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/base/structs/set"
	"github.com/YiuTerran/go-common/network"
	"net"
	"sync"
	"time"
)

type Client struct {
	sync.Mutex
	Addr            string
	ConnNum         int
	ConnectInterval time.Duration
	AutoReconnect   bool
	NewAgentFunc    func(*Conn) network.Session
	Parser          IParser

	cons      *set.Set[net.Conn]
	wg        sync.WaitGroup
	closeFlag bool
}

type Option func(*Client)

func NewClient(addr string, newAgentFunc func(*Conn) network.Session, options ...Option) *Client {
	c := &Client{
		Addr:            addr,
		ConnNum:         1,
		ConnectInterval: 3 * time.Second,
		AutoReconnect:   true,
		Parser:          NewDefaultParser(),
		NewAgentFunc:    newAgentFunc,
		cons:            set.NewSet[net.Conn](),
	}
	for _, option := range options {
		option(c)
	}
	return c
}

func ConnNum(num int) Option {
	return func(client *Client) {
		client.ConnNum = num
	}
}

func ConnectInterval(dr time.Duration) Option {
	return func(client *Client) {
		client.ConnectInterval = dr
	}
}

func Parser(p IParser) Option {
	return func(client *Client) {
		client.Parser = p
	}
}

func (client *Client) Start() {
	client.init()

	for i := 0; i < client.ConnNum; i++ {
		client.wg.Add(1)
		go client.connect()
	}
}

func (client *Client) init() {
	client.Lock()
	defer client.Unlock()

	if client.ConnNum <= 0 {
		client.ConnNum = 1
		log.Debug("invalid ConnNum, reset to %v", client.ConnNum)
	}
	if client.ConnectInterval <= 0 {
		client.ConnectInterval = 3 * time.Second
		log.Debug("invalid ConnectInterval, reset to %v", client.ConnectInterval)
	}
	if client.NewAgentFunc == nil {
		log.Fatal("NewSessionFunc must not be nil")
	}
	if client.cons != nil {
		log.Fatal("client is running")
	}

	client.cons = set.NewSet[net.Conn]()
	client.closeFlag = false

	if client.Parser == nil {
		// msg parser
		msgParser := NewDefaultParser()
		client.Parser = msgParser
	}
}

func (client *Client) dial() net.Conn {
	for {
		conn, err := net.Dial("tcp", client.Addr)
		if err == nil || client.closeFlag {
			return conn
		}

		log.Error("connect to %v error: %v", client.Addr, err)
		time.Sleep(client.ConnectInterval)
		continue
	}
}

func (client *Client) connect() {
	defer client.wg.Done()

reconnect:
	conn := client.dial()
	if conn == nil {
		return
	}

	client.Lock()
	if client.closeFlag {
		client.Unlock()
		_ = conn.Close()
		return
	}
	client.cons.AddItem(conn)
	client.Unlock()

	tcpConn := newConn(conn, client.Parser)
	agent := client.NewAgentFunc(tcpConn)
	agent.Run()

	// cleanup
	tcpConn.Close()
	client.Lock()
	client.cons.RemoveItem(conn)
	client.Unlock()
	agent.OnClose()

	if client.AutoReconnect {
		time.Sleep(client.ConnectInterval)
		goto reconnect
	}
}

func (client *Client) Close() {
	client.Lock()
	client.closeFlag = true
	client.cons.ForEach(func(conn net.Conn) {
		_ = conn.Close()
	})
	client.cons = nil
	client.Unlock()

	client.wg.Wait()
}
