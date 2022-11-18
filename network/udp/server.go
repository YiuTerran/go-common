package udp

import (
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/base/structs/chanx"
	"github.com/YiuTerran/go-common/base/util/byteutil"
	"github.com/YiuTerran/go-common/network"
	"net"
	"strings"
	"sync"
)

type ReceivedContext struct {
	Addr   net.Addr
	Server *Server
}

type MsgInfo struct {
	Addr net.Addr
	Msg  []byte
}

type Server struct {
	Addr      string
	Processor network.MsgProcessor
	//发送失败后尝试次数
	FailTry int

	closeSig  chan struct{}
	readChan  *chanx.UnboundedChan[*MsgInfo]
	writeChan *chanx.UnboundedChan[*MsgInfo]
	conn      net.PacketConn
	wg        *sync.WaitGroup
}

func (server *Server) Start() {
	conn, err := net.ListenPacket("udp", server.Addr)
	if err != nil {
		log.Fatal("fail to bind udp port:%v", err)
	}
	if server.FailTry < 0 {
		server.FailTry = 0
	}
	server.closeSig = make(chan struct{}, 1)
	server.writeChan = chanx.NewUnboundedChan[*MsgInfo](MaxPacketSize)
	server.readChan = chanx.NewUnboundedChan[*MsgInfo](MaxPacketSize)
	server.conn = conn
	server.wg = &sync.WaitGroup{}
	go server.listen()
	go server.doWrite()
	go server.doRead()
	server.wg.Add(3)
}

func (server *Server) WriteMsg(msg any, addr net.Addr) error {
	if bs, err := server.Processor.Marshal(msg); err != nil {
		return err
	} else {
		server.writeChan.In <- &MsgInfo{
			Addr: addr,
			Msg:  byteutil.MergeBytes(bs),
		}
	}
	return nil
}

func (server *Server) doWrite() {
	defer func() {
		if r := recover(); r != nil {
			log.PanicStack("", r)
		}
	}()
	for b := range server.writeChan.Out {
		if b == nil {
			break
		}
		count := server.FailTry
		for count >= 0 {
			_, err := server.conn.WriteTo(b.Msg, b.Addr)
			if err != nil {
				log.Error("fail to write udp chan:%+v", err)
				count--
			} else {
				break
			}
		}
	}
	server.wg.Done()
}

func (server *Server) doRead() {
	defer func() {
		if r := recover(); r != nil {
			log.PanicStack("", r)
		}
	}()
	for b := range server.readChan.Out {
		if b == nil {
			break
		}
		msg, err := server.Processor.Unmarshal(b.Msg)
		if err != nil {
			log.Error("fail to decode udp msg:%v", err)
			continue
		}
		err = server.Processor.Route(msg, &ReceivedContext{
			Addr:   b.Addr,
			Server: server,
		})
		if err != nil {
			log.Error("fail to route udp msg:%v", err)
			continue
		}
	}
	server.wg.Done()
}

func (server *Server) listen() {
	defer func() {
		if r := recover(); r != nil {
			log.PanicStack("", r)
		}
	}()
	for {
		select {
		case <-server.closeSig:
			server.writeChan.In <- nil
			server.readChan.In <- nil
			server.wg.Done()
			return
		default:
			buffer := make([]byte, MaxPacketSize)
			n, addr, err := server.conn.ReadFrom(buffer)
			//这里没有什么特别优雅的处理方案，因为net包没有提供
			if err != nil {
				if !strings.HasSuffix(err.Error(), "use of closed network connection") {
					log.Error("fail to doRead udp msg:%v", err)
				}
				continue
			}
			server.readChan.In <- &MsgInfo{
				Addr: addr,
				Msg:  buffer[:n],
			}
		}
	}
}

func (server *Server) Close() {
	_ = server.conn.Close()
	server.closeSig <- struct{}{}
}

func (server *Server) CloseAndWait() {
	server.Close()
	server.wg.Wait()
}
