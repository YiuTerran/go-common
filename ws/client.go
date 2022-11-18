package ws

import (
	"github.com/gorilla/websocket"
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/base/structs/set"
	"github.com/YiuTerran/go-common/network"
	"sync"
	"time"
)

/**
  *  @author tryao
  *  @date 2022/03/22 11:32
**/

//Client websocket的客户端
type Client struct {
	sync.Mutex
	Addr             string
	ConnNum          int
	ConnectInterval  time.Duration
	MaxMsgLen        uint32
	HandshakeTimeout time.Duration
	AutoReconnect    bool
	NewSessionFunc   func(*Conn) network.Session
	TextFormat       bool

	dialer    websocket.Dialer
	conns     *set.Set[*websocket.Conn]
	wg        sync.WaitGroup
	closeFlag bool
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
	if client.MaxMsgLen <= 0 {
		client.MaxMsgLen = 2048000
		log.Debug("invalid MaxMsgLen, reset to %v", client.MaxMsgLen)
	}
	if client.HandshakeTimeout <= 0 {
		client.HandshakeTimeout = 10 * time.Second
		log.Debug("invalid HandshakeTimeout, reset to %v", client.HandshakeTimeout)
	}
	if client.NewSessionFunc == nil {
		log.Fatal("NewSessionFunc must not be nil")
	}
	if client.conns != nil {
		log.Fatal("client is running")
	}

	client.conns = set.NewSet[*websocket.Conn]()
	client.closeFlag = false
	client.dialer = websocket.Dialer{
		HandshakeTimeout: client.HandshakeTimeout,
	}
}

func (client *Client) dial() *websocket.Conn {
	for {
		conn, _, err := client.dialer.Dial(client.Addr, nil)
		if err == nil || client.closeFlag {
			return conn
		}

		log.Info("connect to %v error: %v", client.Addr, err)
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
	conn.SetReadLimit(int64(client.MaxMsgLen))

	client.Lock()
	if client.closeFlag {
		client.Unlock()
		_ = conn.Close()
		return
	}
	client.conns.AddItem(conn)
	client.Unlock()

	wsConn := newWSConn(conn, client.MaxMsgLen, client.TextFormat)
	agent := client.NewSessionFunc(wsConn)
	agent.Run()

	// cleanup
	wsConn.Close()
	client.Lock()
	client.conns.RemoveItem(conn)
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
	client.conns.ForEach(func(conn *websocket.Conn) {
		_ = conn.Close()
	})
	client.conns = nil
	client.Unlock()
	client.wg.Wait()
}
