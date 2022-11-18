package ua

/**
  *  @author tryao
  *  @date 2022/03/31 14:05
**/
import (
	"context"
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/sip/parser"
	"github.com/YiuTerran/go-common/sip/sip"
	"github.com/YiuTerran/go-common/sip/testutils"
	"github.com/YiuTerran/go-common/sip/transport"
	"net"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GoSIP Node", func() {
	var (
		ua           Node
		client1      net.Conn
		inviteBranch string
		inviteReq    sip.Request
	)

	srvConf := NodeConfig{}
	clientAddr := "127.0.0.1:9001"
	localTarget := transport.NewTarget("127.0.0.1", 5060)
	logger := log.Fields{}

	JustBeforeEach(func() {
		ua = NewNode(srvConf, nil, nil, logger)
		Expect(ua.Listen("udp", localTarget.Addr())).To(Succeed())
		Expect(ua.Listen("tcp", localTarget.Addr())).To(Succeed())
	})

	AfterEach(func() {
		ua.Shutdown()
	})

	It("should call INVITE handler on incoming INVITE request via UDP transport", func() {
		client1 = testutils.CreateClient("udp", localTarget.Addr(), clientAddr)
		defer func() {
			Expect(client1.Close()).To(BeNil())
		}()
		inviteBranch = sip.GenerateBranch()
		inviteReq = testutils.Request([]string{
			"INVITE sip:bob@example.com SIP/2.0",
			"Via: SIP/2.0/UDP " + clientAddr + ";branch=" + inviteBranch,
			"From: \"Alice\" <sip:alice@wonderland.com>;tag=1928301774",
			"To: \"Bob\" <sip:bob@far-far-away.com>",
			"CSeq: 1 INVITE",
			"Content-Length: 0",
			"",
			"",
		})

		wg := new(sync.WaitGroup)

		wg.Add(1)
		ua.OnRequest(sip.INVITE, func(req sip.Request, tx sip.ServerTransaction) {
			defer wg.Done()
			Expect(req.Method()).To(Equal(sip.INVITE))
			Expect(tx.Origin().Method()).To(Equal(sip.INVITE))
		})

		wg.Add(1)
		go func() {
			defer wg.Done()
			testutils.WriteToConn(client1, []byte(inviteReq.String()))
		}()
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()
		Eventually(done, "3s").Should(BeClosed())
	})

	It("should call INVITE handler on incoming INVITE request via TCP transport", func() {
		client1 = testutils.CreateClient("tcp", localTarget.Addr(), clientAddr)
		defer func() {
			Expect(client1.Close()).To(BeNil())
		}()
		inviteReq = testutils.Request([]string{
			"INVITE sip:bob@example.com SIP/2.0",
			"Via: SIP/2.0/TCP " + clientAddr + ";branch=" + sip.GenerateBranch(),
			"From: \"Alice\" <sip:alice@wonderland.com>;tag=1928301774",
			"To: \"Bob\" <sip:bob@far-far-away.com>",
			"CSeq: 1 INVITE",
			"Content-Length: 0",
			"",
			"",
		})

		wg := new(sync.WaitGroup)
		wg.Add(1)
		ua.OnRequest(sip.INVITE, func(req sip.Request, tx sip.ServerTransaction) {
			defer wg.Done()

			Expect(req.Method()).To(Equal(sip.INVITE))
			Expect(tx.Origin().Method()).To(Equal(sip.INVITE))
			_, err := ua.RespondOnRequest(req, 200, "OK", "", nil)
			Expect(err).ShouldNot(HaveOccurred())
		})

		wg.Add(1)
		go func() {
			defer wg.Done()

			_, err := client1.Write([]byte(inviteReq.String()))
			Expect(err).ShouldNot(HaveOccurred())

			buf := make([]byte, 1000)
			n, err := client1.Read(buf)
			Expect(err).ShouldNot(HaveOccurred())
			msg, err := parser.ParseMessage(buf[:n], logger)
			Expect(err).ShouldNot(HaveOccurred())
			res, ok := msg.(sip.Response)
			Expect(ok).Should(BeTrue())
			Expect(int(res.StatusCode())).Should(Equal(200))
		}()
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()
		Eventually(done, "3s").Should(BeClosed())
	})

	It("should send INVITE request through TX layer with UDP transport", func() {
		inviteReq = testutils.Request([]string{
			"INVITE sip:bob@example.com SIP/2.0",
			"Route: <sip:" + clientAddr + ";lr>",
			"From: \"Alice\" <sip:alice@wonderland.com>;tag=1928301774",
			"To: \"Bob\" <sip:bob@far-far-away.com>",
			"CSeq: 1 INVITE",
			"",
			"Hello world!",
		})

		wg := new(sync.WaitGroup)
		wg.Add(1)
		go func() {
			defer wg.Done()

			conn, err := net.ListenPacket("udp", clientAddr)
			Expect(err).ShouldNot(HaveOccurred())
			defer conn.Close()

			buf := make([]byte, transport.MTU)
			i := 0
			for {
				num, raddr, err := conn.ReadFrom(buf)
				if err != nil {
					return
				}

				msg, err := parser.ParseMessage(buf[:num], logger)
				Expect(err).ShouldNot(HaveOccurred())
				viaHop, ok := msg.ViaHop()
				Expect(ok).Should(BeTrue())
				if viaHop.Params == nil {
					viaHop.Params = sip.NewParams(sip.HeaderParams)
				}
				viaHop.Params.Add("received", sip.String{Str: raddr.(*net.UDPAddr).IP.String()})
				req, ok := msg.(sip.Request)
				Expect(ok).Should(BeTrue())
				Expect(req.Method()).Should(Equal(sip.INVITE))
				Expect(req.Recipient().String()).Should(Equal("sip:bob@example.com"))

				// sleep and wait for retransmission
				if i == 0 {
					time.Sleep(300 * time.Millisecond)
					i++
					continue
				}

				res := sip.NewResponseFromRequest("", req, sip.StatusTrying, sip.Phrase(sip.StatusTrying), "")
				raddr, err = net.ResolveUDPAddr("udp", res.Destination())
				Expect(err).ShouldNot(HaveOccurred())
				num, err = conn.WriteTo([]byte(res.String()), raddr)
				Expect(err).ShouldNot(HaveOccurred())

				time.Sleep(100 * time.Millisecond)

				res = sip.NewResponseFromRequest("", req, sip.StatusOK, "OK", "")
				num, err = conn.WriteTo([]byte(res.String()), raddr)
				Expect(err).ShouldNot(HaveOccurred())

				return
			}
		}()

		i := int32(0)
		res, err := ua.RequestWithContext(context.Background(), inviteReq,
			WithResponseHandler(func(res sip.Response, request sip.Request) {
				switch atomic.LoadInt32(&i) {
				case 0:
					Expect(int(res.StatusCode())).Should(Equal(100))
				case 1:
					Expect(int(res.StatusCode())).Should(Equal(200))
					ack := sip.NewAckRequest("", request, res, "", nil)
					Expect(ack).ShouldNot(BeNil())
				}
				atomic.AddInt32(&i, 1)
			}),
		)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(int(res.StatusCode())).Should(Equal(200))
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()
		Eventually(done, "3s").Should(BeClosed())
	})

	It("should send INVITE request through TX layer with TCP transport", func() {
		inviteReq = testutils.Request([]string{
			"INVITE sip:bob@example.com SIP/2.0",
			"Route: <sip:" + clientAddr + ";transport=tcp;lr>",
			"From: \"Alice\" <sip:alice@wonderland.com>;tag=1928301774",
			"To: \"Bob\" <sip:bob@far-far-away.com>",
			"CSeq: 1 INVITE",
			"",
			"Hello world!",
		})

		wg := new(sync.WaitGroup)
		wg.Add(1)
		go func() {
			defer wg.Done()

			server, err := net.Listen("tcp", clientAddr)
			Expect(err).ShouldNot(HaveOccurred())
			defer server.Close()

			conn, err := server.Accept()
			Expect(err).ShouldNot(HaveOccurred())
			defer conn.Close()

			buf := make([]byte, transport.MTU)
			for {
				num, err := conn.Read(buf)
				if err != nil {
					return
				}

				msg, err := parser.ParseMessage(buf[:num], logger)
				Expect(err).ShouldNot(HaveOccurred())
				viaHop, ok := msg.ViaHop()
				Expect(ok).Should(BeTrue())
				if viaHop.Params == nil {
					viaHop.Params = sip.NewParams(sip.HeaderParams)
				}
				viaHop.Params.Add("received", sip.String{Str: conn.RemoteAddr().(*net.TCPAddr).IP.String()})
				req, ok := msg.(sip.Request)
				Expect(ok).Should(BeTrue())
				Expect(req.Method()).Should(Equal(sip.INVITE))
				Expect(req.Recipient().String()).Should(Equal("sip:bob@example.com"))

				time.Sleep(100 * time.Millisecond)

				res := sip.NewResponseFromRequest("", req, 100, "Trying", "")
				res.SetBody("", true)
				num, err = conn.Write([]byte(res.String()))
				Expect(err).ShouldNot(HaveOccurred())

				time.Sleep(100 * time.Millisecond)

				res = sip.NewResponseFromRequest("", req, 200, "Ok", "")
				res.SetBody("", true)
				num, err = conn.Write([]byte(res.String()))
				Expect(err).ShouldNot(HaveOccurred())

				return
			}
		}()

		i := int32(0)
		res, err := ua.RequestWithContext(context.Background(), inviteReq,
			WithResponseHandler(func(res sip.Response, request sip.Request) {
				switch atomic.LoadInt32(&i) {
				case 0:
					Expect(int(res.StatusCode())).Should(Equal(100))
				case 1:
					Expect(int(res.StatusCode())).Should(Equal(200))
					ack := sip.NewAckRequest("", request, res, "", nil)
					Expect(ack).ShouldNot(BeNil())
				}
				atomic.AddInt32(&i, 1)
			}),
		)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(int(res.StatusCode())).Should(Equal(200))
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()
		Eventually(done, "3s").Should(BeClosed())
	})
})
