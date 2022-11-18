package transaction_test

import (
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/sip/sip"
	"github.com/YiuTerran/go-common/sip/testutils"
	"github.com/YiuTerran/go-common/sip/transaction"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ClientTx", func() {
	var (
		tpl *testutils.MockTransportLayer
		txl transaction.Layer
	)

	clientAddr := "localhost:9001"

	BeforeEach(func() {
		tpl = testutils.NewMockTransportLayer()
		txl = transaction.NewLayer(tpl, log.Fields{})
	})
	AfterEach(func() {
		done := make(chan any)
		go func() {
			<-txl.Done()
			close(done)
		}()
		txl.Cancel()
		Eventually(done, "3s").Should(BeClosed())
	})

	Context("just initialized", func() {
		It("should has transport layer", func() {
			Expect(txl.Transport()).To(Equal(tpl))
		})
	})

	Context("sends INVITE request", func() {
		var err error
		var invite, trying, ok, notOk, ack, canceled sip.Message
		var inviteBranch string
		var tx sip.ClientTransaction

		mu := new(sync.Mutex)

		BeforeEach(func() {
			inviteBranch = sip.GenerateBranch()
			invite = testutils.Request([]string{
				"INVITE sip:bob@example.com SIP/2.0",
				"Via: SIP/2.0/UDP " + clientAddr + ";branch=" + inviteBranch,
				"CSeq: 1 INVITE",
				"",
				"",
			})
			trying = testutils.Response([]string{
				"SIP/2.0 100 Trying",
				"Via: SIP/2.0/UDP " + clientAddr + ";branch=" + inviteBranch,
				"CSeq: 1 INVITE",
				"",
				"",
			})
			ok = testutils.Response([]string{
				"SIP/2.0 200 OK",
				"CSeq: 1 INVITE",
				"Via: SIP/2.0/UDP " + clientAddr + ";branch=" + inviteBranch,
				"",
				"",
			})
			notOk = testutils.Response([]string{
				"SIP/2.0 400 Bad Request",
				"CSeq: 1 INVITE",
				"Via: SIP/2.0/UDP " + clientAddr + ";branch=" + inviteBranch,
				"",
				"",
			})
			canceled = testutils.Response([]string{
				"SIP/2.0 487 Request Terminated",
				"CSeq: 1 INVITE",
				"Via: SIP/2.0/UDP " + clientAddr + ";branch=" + inviteBranch,
				"",
				"",
			})
			ack = testutils.Request([]string{
				"ACK sip:bob@example.com SIP/2.0",
				"Via: SIP/2.0/UDP " + clientAddr + ";branch=" + sip.GenerateBranch(),
				"CSeq: 1 ACK",
				"",
				"",
			})
			_ = testutils.Request([]string{
				"ACK sip:bob@example.com SIP/2.0",
				"Via: SIP/2.0/UDP " + clientAddr + ";branch=" + inviteBranch,
				"CSeq: 1 ACK",
				"",
				"",
			})
		})

		It("should send INVITE request", func() {
			go func() {
				msg := <-tpl.OutMsgs
				Expect(msg).ToNot(BeNil())
				Expect(msg.String()).To(Equal(invite.String()))
			}()

			_, err = transaction.MakeClientTxKey(invite)
			Expect(err).ToNot(HaveOccurred())
			_, err = transaction.MakeClientTxKey(ack)
			Expect(err).ToNot(HaveOccurred())

			mu.Lock()
			tx, err = txl.Request(invite.(sip.Request))
			Expect(tx).ToNot(BeNil())
			Expect(err).ToNot(HaveOccurred())
			mu.Unlock()
		})

		Context("receives 200 OK on INVITE", func() {
			wg := new(sync.WaitGroup)
			BeforeEach(func() {
				wg.Add(1)
				go func() {
					defer wg.Done()
					msg := <-tpl.OutMsgs
					Expect(msg).ToNot(BeNil())
					Expect(msg.String()).To(Equal(invite.String()))

					time.Sleep(200 * time.Millisecond)
					tpl.InMsgs <- trying

					time.Sleep(200 * time.Millisecond)
					tpl.InMsgs <- ok
				}()

				mu.Lock()
				tx, err = txl.Request(invite.(sip.Request))
				Expect(tx).ToNot(BeNil())
				Expect(err).ToNot(HaveOccurred())
				mu.Unlock()
			})
			AfterEach(func() {
				done := make(chan any)
				go func() {
					wg.Wait()
					close(done)
				}()
				Eventually(done, "3s").Should(BeClosed())
			})

			It("should receive responses in INVITE tx", func() {
				var msg sip.Response
				msg = <-tx.Responses()
				Expect(msg).ToNot(BeNil())
				Expect(msg.String()).To(Equal(trying.String()))

				msg = <-tx.Responses()
				Expect(msg).ToNot(BeNil())
				Expect(msg.String()).To(Equal(ok.String()))
			})
		})

		Context("receives 400 Bad Request on INVITE", func() {
			wg := new(sync.WaitGroup)

			BeforeEach(func() {
				wg.Add(1)
				go func() {
					defer wg.Done()
					var msg sip.Message
					msg = <-tpl.OutMsgs
					Expect(msg).ToNot(BeNil())
					Expect(msg.String()).To(Equal(invite.String()))

					time.Sleep(200 * time.Millisecond)
					tpl.InMsgs <- trying

					time.Sleep(200 * time.Millisecond)
					tpl.InMsgs <- notOk

					msg = <-tpl.OutMsgs
					Expect(msg).ToNot(BeNil())
					req, ok := msg.(sip.Request)
					Expect(ok).To(BeTrue())
					Expect(string(req.Method())).To(Equal("ACK"))
				}()

				mu.Lock()
				tx, err = txl.Request(invite.(sip.Request))
				Expect(tx).ToNot(BeNil())
				Expect(err).ToNot(HaveOccurred())
				mu.Unlock()
			})
			AfterEach(func() {
				done := make(chan any)
				go func() {
					wg.Wait()
					close(done)
				}()
				Eventually(done, "3s").Should(BeClosed())
			})

			It("should receive responses in INVITE tx and send ACK", func() {
				var msg sip.Response
				msg = <-tx.Responses()
				Expect(msg).ToNot(BeNil())
				Expect(msg.String()).To(Equal(trying.String()))

				msg = <-tx.Responses()
				Expect(msg).ToNot(BeNil())
				Expect(msg.String()).To(Equal(notOk.String()))
			})
		})

		Context("cancel INVITE", func() {
			wg := new(sync.WaitGroup)
			BeforeEach(func() {
				wg.Add(1)
				go func() {
					defer wg.Done()

					var msg sip.Message
					msg = <-tpl.OutMsgs
					Expect(msg).ToNot(BeNil())
					Expect(msg.String()).To(Equal(invite.String()))

					time.Sleep(200 * time.Millisecond)
					tpl.InMsgs <- trying

					msg = <-tpl.OutMsgs
					Expect(msg).ToNot(BeNil())
					req, ok := msg.(sip.Request)
					Expect(ok).To(BeTrue())
					Expect(string(req.Method())).To(Equal("CANCEL"))

					time.Sleep(10 * time.Millisecond)
					tpl.InMsgs <- canceled

					msg = <-tpl.OutMsgs
					Expect(msg).ToNot(BeNil())
					req, ok = msg.(sip.Request)
					Expect(ok).To(BeTrue())
					Expect(string(req.Method())).To(Equal("ACK"))
				}()

				mu.Lock()
				tx, err = txl.Request(invite.(sip.Request))
				Expect(tx).ToNot(BeNil())
				Expect(err).ToNot(HaveOccurred())
				mu.Unlock()
			})
			AfterEach(func() {
				done := make(chan any)
				go func() {
					wg.Wait()
					close(done)
				}()
				Eventually(done, "3s").Should(BeClosed())
			})

			It("should send CANCEL request", func() {
				var msg sip.Response
				msg = <-tx.Responses()
				Expect(msg).ToNot(BeNil())
				Expect(msg.String()).To(Equal(trying.String()))

				Expect(tx.Cancel()).Should(Succeed())

				msg = <-tx.Responses()
				Expect(msg).ToNot(BeNil())
				Expect(msg.String()).To(Equal(canceled.String()))
			})
		})
	})
})
