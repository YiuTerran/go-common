package udp

import (
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/base/util/byteutil"
	"github.com/YiuTerran/go-common/network"
	"net"
	"time"
)

// Client 无连接的一次性的udp client，用于同步处理，可以类似HTTP客户端那种使用方式
// UDP通信流程有两种方式，一种类似TCP，也可以bind然后双向通信. 不同的是UDP的connect啥也不做，只是在本地建立了一个五元组映射。此时内核会维护这套
// 映射，一旦收到消息就转发给五元组中的本地端口。直到应用程序明确释放掉这个绑定。使用`DialUDP`建立*连接*。
// 另外一种是无连接的方式，内核将数据包发出去之后，就直接释放掉。使用`ListenUDP`获取的地址直接发送。
type Client struct {
	serverAddr *net.UDPAddr
	processor  network.MsgProcessor
	conn       *net.UDPConn
}

// NewClient 生成一个指定的客户端
func NewClient(addr string, processor network.MsgProcessor) *Client {
	rAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		log.Error("fail to resolve udp addr %s", addr)
		return nil
	}
	var conn *net.UDPConn
	if conn, err = net.ListenUDP("udp", nil); err != nil {
		log.Error("fail to listen udp")
		return nil
	}
	return &Client{
		serverAddr: rAddr,
		processor:  processor,
		conn:       conn,
	}
}

// Request 同步请求并等待响应
func (c *Client) Request(msg any, timeout time.Duration) (resp any, err error) {
	if err = c.Push(msg); err != nil {
		return
	}
	if timeout > 0 {
		_ = c.conn.SetDeadline(time.Now().Add(timeout))
	}
	buffer := make([]byte, MaxPacketSize)
	var n int
	n, _, err = c.conn.ReadFromUDP(buffer)
	if err != nil {
		return
	}
	resp, err = c.processor.Unmarshal(buffer[:n])
	return
}

// Push 直接推送，不看结果
func (c *Client) Push(msg any) error {
	bs, err := c.processor.Marshal(msg)
	if err != nil {
		return err
	}
	if _, err = c.conn.WriteToUDP(byteutil.MergeBytes(bs), c.serverAddr); err != nil {
		return err
	}
	return nil
}

// Close 关闭客户端
func (c *Client) Close() {
	_ = c.conn.Close()
}
