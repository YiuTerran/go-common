package transport_test

import (
	"fmt"
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/sip/testutils"
	"github.com/YiuTerran/go-common/sip/transport"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Connection", func() {
	logger := log.Fields{}

	Describe("construct", func() {
		Context("from net.UDPConn", func() {
			It("should set connection params", func() {
				cUdpConn, sUdpConn := testutils.CreatePacketClientServer("udp", localAddr1)
				defer func() {
					_ = cUdpConn.Close()
					_ = sUdpConn.Close()
				}()
				conn := transport.NewConnection(sUdpConn, "dummy", "udp", logger)

				Expect(conn.Network()).To(Equal("UDP"))
				Expect(conn.Streamed()).To(BeFalse(), "UDP should be non-streamed")
				Expect(conn.LocalAddr().String()).To(Equal(sUdpConn.LocalAddr().String()))

				if err := conn.Close(); err != nil {
					Fail(err.Error())
				}
			})
		})

		Context("from net.TCPConn", func() {
			It("should set connection params", func() {
				cTcpConn, sTcpConn := testutils.CreateStreamClientServer("tcp", localAddr1)
				defer func() {
					_ = cTcpConn.Close()
					_ = sTcpConn.Close()
				}()
				conn := transport.NewConnection(sTcpConn, "dummy", "tcp", logger)

				Expect(conn.Network()).To(Equal("TCP"))
				Expect(conn.Streamed()).To(BeTrue())
				Expect(conn.LocalAddr().String()).To(Equal(sTcpConn.LocalAddr().String()))
				Expect(conn.RemoteAddr().String()).To(Equal(sTcpConn.RemoteAddr().String()))

				if err := conn.Close(); err != nil {
					Fail(err.Error())
				}
			})
		})
	})

	Describe("read and write", func() {
		data := "Hello world!"

		Context("with net.UDPConn", func() {
			It("should read and write data", func() {
				cUdpConn, sUdpConn := testutils.CreatePacketClientServer("udp", localAddr1)
				defer func() {
					_ = cUdpConn.Close()
					_ = sUdpConn.Close()
				}()

				sConn := transport.NewConnection(sUdpConn, "dummy", "udp", logger)
				cConn := transport.NewConnection(cUdpConn, "dummy", "udp", logger)

				wg := new(sync.WaitGroup)
				wg.Add(1)
				go func() {
					defer wg.Done()

					buf := make([]byte, 65535)
					num, raddr, err := sConn.ReadFrom(buf)
					Expect(err).ToNot(HaveOccurred())
					Expect(fmt.Sprintf("%v", raddr)).To(Equal(fmt.Sprintf("%v", cConn.LocalAddr())))
					Expect(string(buf[:num])).To(Equal(data))
				}()

				num, err := cConn.Write([]byte(data))
				Expect(err).ToNot(HaveOccurred())
				Expect(num).To(Equal(len(data)))
				wg.Wait()
			})
		})
		// TODO: add TCP test
	})
})
