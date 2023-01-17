package ua

import (
	"context"
	"errors"
	"fmt"
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/sip/sip"
	"github.com/YiuTerran/go-common/sip/transaction"
	"github.com/YiuTerran/go-common/sip/transport"
	"go.uber.org/atomic"
	"io"
	"net"
	"os"
	"sync"
	"syscall"
)

/**  SIP结点并不区分客户端或者服务器，标准文档里称为UA，所以这里就叫node更加合理
  *  无论作为客户端还是服务器，都需要监听一个端口，因为需要允许任一端发起反向通信
  *  @author tryao
  *  @date 2022/03/31 14:03
**/

// RequestHandler is a callback that will be called on the incoming request
// of the certain method. tx argument may be nil for 2xx ACK request.
type RequestHandler func(req sip.Request, tx sip.ServerTransaction)

type Node interface {
	Shutdown()
	// Listen 某个端口充当服务器，由于sip节点不可能单独作为客户端或者服务器，所以
	// 所有的结点都要监听某个端口作为服务，这里没有显式的Connect作为客户端，而是在
	// Request的时候再建立连接，tcp会复用连接放在连接池里面
	Listen(network, addr string, options ...transport.ListenOption) error
	// Send 非事务消息可以使用Send直接发送
	Send(msg sip.Message) error
	// Request 事务请求，返回客户端事务指针
	Request(req sip.Request) (sip.ClientTransaction, error)
	// RequestWithContext 事务请求，可以在options里面加入回调函数和认证函数
	RequestWithContext(
		ctx context.Context,
		request sip.Request,
		options ...RequestWithContextOption,
	) (sip.Response, error)
	// OnRequest 注册各种方法的回调函数
	OnRequest(method sip.RequestMethod, handler RequestHandler)
	// Respond 事务响应，返回服务端事务指针
	Respond(res sip.Response) (sip.ServerTransaction, error)
	// RespondOnRequest 根据事务请求生成事务响应，返回服务端事务指针
	RespondOnRequest(
		request sip.Request,
		status sip.StatusCode,
		reason, body string,
		headers []sip.Header,
	) (sip.ServerTransaction, error)
}

type TransportLayerFactory func(
	ip net.IP,
	dnsResolver *net.Resolver,
	msgMapper sip.MessageMapper,
	fields log.Fields,
) transport.Layer

type TransactionLayerFactory func(tpl sip.Transport, logger log.Fields) transaction.Layer

// NodeConfig describes available options
type NodeConfig struct {
	// Public IP address or domain name, if empty use localhost
	Host string
	// Dns is an address of the public DNS node to use in SRV lookup.
	Dns        string
	Extensions []string
	MsgMapper  sip.MessageMapper
	UserAgent  string
}

// Node is a SIP node
type node struct {
	running         atomic.Bool
	tp              transport.Layer
	tx              transaction.Layer
	ip              net.IP
	hwg             *sync.WaitGroup
	hmu             *sync.RWMutex
	requestHandlers map[sip.RequestMethod]RequestHandler
	extensions      []string
	userAgent       string

	fields log.Fields
}

// NewDefaultNode 默认的sip服务
func NewDefaultNode(config NodeConfig) Node {
	return NewNode(config, nil, nil, log.Fields{})
}

// NewNode creates new instance of SIP node.
func NewNode(
	config NodeConfig,
	tpFactory TransportLayerFactory,
	txFactory TransactionLayerFactory,
	fields log.Fields,
) Node {
	if tpFactory == nil {
		tpFactory = transport.NewLayer
	}
	if txFactory == nil {
		txFactory = transaction.NewLayer
	}

	fields = fields.WithPrefix("gosip.Node")

	var host string
	var ip net.IP
	if config.Host == "" {
		config.Host = "127.0.0.1"
	}
	host = config.Host
	if addr, err := net.ResolveIPAddr("ip", host); err == nil {
		ip = addr.IP
	} else {
		fields.Fatal("resolve host IP failed: %s", err)
	}
	var dnsResolver *net.Resolver
	if config.Dns != "" {
		dnsResolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{}
				return d.DialContext(ctx, "udp", config.Dns)
			},
		}
	} else {
		dnsResolver = net.DefaultResolver
	}

	var extensions []string
	if config.Extensions != nil {
		extensions = config.Extensions
	}

	userAgent := config.UserAgent
	if userAgent == "" {
		userAgent = "GoSIP"
	}

	nd := &node{
		ip:              ip,
		hwg:             new(sync.WaitGroup),
		hmu:             new(sync.RWMutex),
		requestHandlers: make(map[sip.RequestMethod]RequestHandler),
		extensions:      extensions,
		userAgent:       userAgent,
	}
	nd.fields = fields.WithFields(log.Fields{
		"sip_server_ptr": fmt.Sprintf("%p", nd),
	})
	nd.tp = tpFactory(ip, dnsResolver, config.MsgMapper, nd.Fields())
	sipTp := &sipTransport{
		tpl: nd.tp,
		srv: nd,
	}
	nd.tx = txFactory(sipTp, nd.fields)

	nd.running.Store(true)
	go nd.serve()

	return nd
}

func (nd *node) Fields() log.Fields {
	return nd.fields
}

// Listen starts serving listeners on the provided address
func (nd *node) Listen(network string, listenAddr string, options ...transport.ListenOption) error {
	return nd.tp.Listen(network, listenAddr, options...)
}

func (nd *node) serve() {
	defer nd.Shutdown()

	for {
		select {
		case tx, ok := <-nd.tx.Requests():
			if !ok {
				return
			}
			nd.hwg.Add(1)
			go nd.handleRequest(tx.Origin(), tx)
		case ack, ok := <-nd.tx.Acks():
			if !ok {
				return
			}
			nd.hwg.Add(1)
			go nd.handleRequest(ack, nil)
		case response, ok := <-nd.tx.Responses():
			if !ok {
				return
			}
			logger := nd.Fields().WithFields(response.Fields())
			logger.Warn("received not matched response")
		case err, ok := <-nd.tx.Errors():
			if !ok {
				return
			}
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) {
				nd.Fields().Debug("received SIP transaction error: %s", err)
			} else {
				nd.Fields().Error("received SIP transaction error: %s", err)
			}
		case err, ok := <-nd.tp.Errors():
			if !ok {
				return
			}
			var fErr *sip.MalformedMessageError
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) ||
				errors.Is(err, syscall.ECONNRESET) || os.IsTimeout(err) {
				//ignore
			} else if errors.As(err, &fErr) {
				nd.Fields().Warn("received SIP transport error: %s", err)
			} else {
				nd.Fields().Error("received SIP transport error: %s", err)
			}
		}
	}
}

// 每个请求一个独立处理协程
func (nd *node) handleRequest(req sip.Request, tx sip.ServerTransaction) {
	defer nd.hwg.Done()

	nd.hmu.RLock()
	handler, ok := nd.requestHandlers[req.Method()]
	nd.hmu.RUnlock()

	if !ok {
		if tx != nil && !req.IsAck() {
			_ = tx.Respond(sip.NewResponseFromRequest(
				"", req, 405, "Method Not Allowed", ""))
		}
		return
	}
	handler(req, tx)
}

// Request Send SIP request and return a client transaction
func (nd *node) Request(req sip.Request) (sip.ClientTransaction, error) {
	if !nd.running.Load() {
		return nil, fmt.Errorf("can not send through stopped node")
	}

	return nd.tx.Request(nd.prepareRequest(req))
}

func (nd *node) RequestWithContext(
	ctx context.Context,
	request sip.Request,
	options ...RequestWithContextOption,
) (sip.Response, error) {
	return nd.requestWithContext(ctx, request, 1, options...)
}

func (nd *node) requestWithContext(
	ctx context.Context,
	request sip.Request,
	attempt int,
	options ...RequestWithContextOption,
) (sip.Response, error) {
	tx, err := nd.Request(request)
	if err != nil {
		return nil, err
	}

	optionsHash := &RequestWithContextOptions{}
	for _, opt := range options {
		opt.ApplyRequestWithContext(optionsHash)
	}

	txResponses := tx.Responses()
	txErrs := tx.Errors()
	responses := make(chan sip.Response, 1)
	errs := make(chan error, 1)
	done := make(chan struct{})
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()

		select {
		case <-done:
		case <-ctx.Done():
			//invite的cancel，按rfc执行流程；非invite实际上不接受外部传入的超时，而是按着rfc规定的定时器来
			if err := tx.Cancel(); err != nil {
				nd.Fields().Error("cancel transaction failed", log.Fields{
					"transaction_key": tx.Key(),
				})
			}
		}
	}()
	go func() {
		defer func() {
			close(done)
			wg.Done()
		}()

		var lastResponse sip.Response

		previousMessages := make([]sip.Response, 0)
		previousResponsesStatuses := make(map[string]bool)
		getKey := func(res sip.Response) string {
			return fmt.Sprintf("%d %s", res.StatusCode(), res.Reason())
		}

		for {
			select {
			case err, ok := <-txErrs:
				if !ok {
					txErrs = nil
					// errors chan was closed
					// we continue to pull responses until close
					continue
				}
				errs <- err
				return
			case response, ok := <-txResponses:
				if !ok {
					if lastResponse != nil {
						lastResponse.SetPrevious(previousMessages)
					}
					errs <- sip.NewRequestError(487, "Request Terminated", request, lastResponse)
					return
				}

				response = sip.CopyResponse(response)
				lastResponse = response

				if optionsHash.ResponseHandler != nil {
					optionsHash.ResponseHandler(response, request)
				}

				if response.IsProvisional() {
					if _, ok := previousResponsesStatuses[getKey(response)]; !ok {
						previousMessages = append(previousMessages, response)
						previousResponsesStatuses[getKey(response)] = true
					}

					continue
				}

				// success
				if response.IsSuccess() {
					response.SetPrevious(previousMessages)
					responses <- response

					go func() {
						for response := range tx.Responses() {
							if optionsHash.ResponseHandler != nil {
								optionsHash.ResponseHandler(response, request)
							}
						}
					}()

					return
				}

				// need auth request
				needAuth := (response.StatusCode() == 401 || response.StatusCode() == 407) && attempt < 2
				if needAuth && optionsHash.Authorizer != nil {
					if err := optionsHash.Authorizer.AuthorizeRequest(request, response); err != nil {
						errs <- err
						return
					}
					if response, err := nd.requestWithContext(ctx, request, attempt+1, options...); err == nil {
						responses <- response
					} else {
						errs <- err
					}

					return
				}

				// failed request
				response.SetPrevious(previousMessages)
				errs <- sip.NewRequestError(uint(response.StatusCode()), response.Reason(), request, response)

				return
			}
		}
	}()

	var res sip.Response
	select {
	case err = <-errs:
	case res = <-responses:
	}

	wg.Wait()

	return res, err
}

func (nd *node) prepareRequest(req sip.Request) sip.Request {
	nd.appendAutoHeaders(req)

	return req
}

func (nd *node) Respond(res sip.Response) (sip.ServerTransaction, error) {
	if !nd.running.Load() {
		return nil, fmt.Errorf("can not send through stopped node")
	}

	return nd.tx.Respond(nd.prepareResponse(res))
}

func (nd *node) RespondOnRequest(
	request sip.Request,
	status sip.StatusCode,
	reason, body string,
	headers []sip.Header,
) (sip.ServerTransaction, error) {
	response := sip.NewResponseFromRequest("", request, status, reason, body)
	for _, header := range headers {
		response.AppendHeader(header)
	}

	tx, err := nd.Respond(response)
	if err != nil {
		return nil, fmt.Errorf("respond '%d %s' failed: %w", response.StatusCode(), response.Reason(), err)
	}

	return tx, nil
}

func (nd *node) Send(msg sip.Message) error {
	if !nd.running.Load() {
		return fmt.Errorf("can not send through stopped node")
	}

	switch m := msg.(type) {
	case sip.Request:
		msg = nd.prepareRequest(m)
	case sip.Response:
		msg = nd.prepareResponse(m)
	}

	return nd.tp.Send(msg)
}

func (nd *node) prepareResponse(res sip.Response) sip.Response {
	nd.appendAutoHeaders(res)

	return res
}

// Shutdown gracefully shutdowns SIP node
func (nd *node) Shutdown() {
	if !nd.running.CompareAndSwap(true, false) {
		return
	}
	// stop transaction layer
	nd.tx.Cancel()
	<-nd.tx.Done()
	// stop transport layer
	nd.tp.Cancel()
	<-nd.tp.Done()
	// wait for handlers
	nd.hwg.Wait()
}

// OnRequest registers new request callback
func (nd *node) OnRequest(method sip.RequestMethod, handler RequestHandler) {
	nd.hmu.Lock()
	nd.requestHandlers[method] = handler
	nd.hmu.Unlock()
}

func (nd *node) appendAutoHeaders(msg sip.Message) {
	autoAppendMethods := map[sip.RequestMethod]bool{
		sip.OPTIONS: true,
		//sip.INVITE:   true,
		//sip.REGISTER: true,
		//sip.REFER:    true,
		//sip.NOTIFY:   true,
	}

	var msgMethod sip.RequestMethod
	switch m := msg.(type) {
	case sip.Request:
		msgMethod = m.Method()
	case sip.Response:
		if cseq, ok := m.CSeq(); ok && !m.IsProvisional() {
			msgMethod = cseq.MethodName
		}
	}
	if len(msgMethod) > 0 {
		if _, ok := autoAppendMethods[msgMethod]; ok {
			headers := msg.GetHeaders("Allow")
			if len(headers) == 0 {
				allow := make(sip.AllowHeader, 0)
				for _, method := range nd.getAllowedMethods() {
					allow = append(allow, method)
				}

				if len(allow) > 0 {
					msg.AppendHeader(allow)
				}
			}

			headers = msg.GetHeaders("Supported")
			if len(headers) == 0 && len(nd.extensions) > 0 {
				msg.AppendHeader(&sip.SupportedHeader{
					Options: nd.extensions,
				})
			}
		}
	}

	if hdrs := msg.GetHeaders("User-Agent"); len(hdrs) == 0 {
		userAgent := sip.UserAgentHeader(nd.userAgent)
		msg.AppendHeader(&userAgent)
	}

	if hdrs := msg.GetHeaders("Content-Length"); len(hdrs) == 0 {
		msg.SetBody(msg.Body(), true)
	}
}

func (nd *node) getAllowedMethods() []sip.RequestMethod {
	methods := []sip.RequestMethod{
		sip.INVITE,
		sip.ACK,
		sip.CANCEL,
	}
	added := map[sip.RequestMethod]bool{
		sip.INVITE: true,
		sip.ACK:    true,
		sip.CANCEL: true,
	}

	nd.hmu.RLock()
	for method := range nd.requestHandlers {
		if _, ok := added[method]; !ok {
			methods = append(methods, method)
		}
	}
	nd.hmu.RUnlock()

	return methods
}

type sipTransport struct {
	tpl transport.Layer
	srv *node
}

func (tp *sipTransport) Messages() <-chan sip.Message {
	return tp.tpl.Messages()
}

func (tp *sipTransport) Send(msg sip.Message) error {
	return tp.srv.Send(msg)
}

func (tp *sipTransport) IsReliable(network string) bool {
	return tp.tpl.IsReliable(network)
}

func (tp *sipTransport) IsStreamed(network string) bool {
	return tp.tpl.IsStreamed(network)
}
