package udp

import (
	"errors"
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/base/structs/chanx"
	"github.com/YiuTerran/go-common/base/util/byteutil"
	"github.com/YiuTerran/go-common/network"
	"net"
	"sync"

	"go.uber.org/atomic"
)

//这是一个异步的udp客户端，代码和服务端很类似，这里调用了DialUDP，因此视为"有连接的"
//如果需要做一些同步操作，最好使用Client，而不是这个版本

const (
	NotInit = 0
	Inited  = 1
	Closed  = 2
)

type AsyncClient struct {
	ServerAddr string
	FailTry    int //失败后尝试次数，默认不尝试
	Processor  network.MsgProcessor

	closeSig  chan struct{}
	writeChan *chanx.UnboundedChan[[]byte]
	readChan  *chanx.UnboundedChan[[]byte]
	conn      *net.UDPConn
	wg        *sync.WaitGroup
	status    atomic.Int32
}

func (client *AsyncClient) Start() error {
	if !client.status.CompareAndSwap(NotInit, Inited) {
		return errors.New("server inited")
	}
	if client.FailTry < 0 {
		client.FailTry = 0
	}
	if client.Processor == nil {
		log.Fatal("udp client no processor registered!")
	}
	client.closeSig = make(chan struct{}, 1)
	client.writeChan = chanx.NewUnboundedChan[[]byte](MaxPacketSize)
	client.readChan = chanx.NewUnboundedChan[[]byte](MaxPacketSize)
	client.wg = &sync.WaitGroup{}

	client.conn = client.dial()
	if client.conn == nil {
		return InitError
	}
	go client.doWrite()
	go client.listen()
	go client.doRead()
	client.wg.Add(3)
	return nil
}

func (client *AsyncClient) listen() {
	defer func() {
		if r := recover(); r != nil {
			log.PanicStack("", r)
		}
	}()
	for {
		select {
		case <-client.closeSig:
			client.readChan.In <- nil
			client.writeChan.In <- nil
			client.wg.Done()
			return
		default:
			buffer := make([]byte, MaxPacketSize)
			n, err := client.conn.Read(buffer)
			if err != nil {
				continue
			}
			buffer = buffer[:n]
			client.readChan.In <- buffer
		}
	}
}

func (client *AsyncClient) doWrite() {
	defer func() {
		if r := recover(); r != nil {
			log.PanicStack("", r)
		}
	}()
	for b := range client.writeChan.Out {
		if b == nil {
			break
		}
		count := client.FailTry
		for count >= 0 {
			_, err := client.conn.Write(b)
			if err != nil {
				log.Error("fail to write udp chan:%v", err)
			} else {
				break
			}
			count--
		}
	}
	client.wg.Done()
}

func (client *AsyncClient) doRead() {
	defer func() {
		if r := recover(); r != nil {
			log.PanicStack("", r)
		}
	}()
	for b := range client.readChan.Out {
		if b == nil {
			break
		}
		msg, err := client.Processor.Unmarshal(b)
		if err != nil {
			log.Error("unable to unmarshal udp msg, ignore")
			continue
		}
		//依靠processor路由异步处理
		err = client.Processor.Route(msg, client)
		if err != nil {
			log.Error("fail to route udp msg:%v", err)
			continue
		}
	}
	client.wg.Done()
}

func (client *AsyncClient) dial() *net.UDPConn {
	rAddr, err := net.ResolveUDPAddr("udp", client.ServerAddr)
	if err != nil {
		log.Error("fail to resolve add for udp:%v", client.ServerAddr)
		return nil
	}
	conn, err := net.DialUDP("udp", nil, rAddr)
	if err == nil {
		return conn
	}

	log.Error("connect to %v error: %v", client.ServerAddr, err)
	return nil
}

func (client *AsyncClient) WriteMsg(msg any) error {
	args, err := client.Processor.Marshal(msg)
	if err != nil {
		return err
	}
	if client.status.Load() == Closed {
		return errors.New("client closed")
	}
	client.writeChan.In <- byteutil.MergeBytes(args)
	return nil
}

func (client *AsyncClient) Close() {
	if !client.status.CompareAndSwap(Inited, Closed) {
		return
	}
	_ = client.conn.Close()
	client.closeSig <- struct{}{}
}

func (client *AsyncClient) CloseAndWait() {
	client.Close()
	client.wg.Wait()
}

func (client *AsyncClient) Destroy() {
	client.status.Store(Closed)
	_ = client.conn.Close()
}
