package transport

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/base/structs/timing"
	"github.com/YiuTerran/go-common/sip/parser"
	"github.com/YiuTerran/go-common/sip/sip"
	"net"
	"sync"
	"time"
)

type ConnectionKey string

func (key ConnectionKey) String() string {
	return string(key)
}

// ConnectionPool used for active connection management.
type ConnectionPool interface {
	Done() <-chan struct{}
	String() string
	Put(connection Connection, ttl time.Duration) error
	Get(key ConnectionKey) (Connection, error)
	All() []Connection
	Drop(key ConnectionKey) error
	DropAll() error
	Length() int
}

// ConnectionHandler serves associated connection, i.e. parses
// incoming data, manages expiry time & etc.
type ConnectionHandler interface {
	Cancel()
	Done() <-chan struct{}
	String() string
	Key() ConnectionKey
	Connection() Connection
	// Expiry returns connection expiry time.
	Expiry() time.Time
	Expired() bool
	// Serve Manage runs connection serving.
	Serve()
	Fields() log.Fields
}

type connectionPool struct {
	store     map[ConnectionKey]ConnectionHandler
	msgMapper sip.MessageMapper

	output chan<- sip.Message
	errs   chan<- error
	cancel <-chan struct{}

	done  chan struct{}
	hMsg  chan sip.Message
	hErrs chan error

	hwg sync.WaitGroup
	mu  sync.RWMutex

	fields log.Fields
}

func NewConnectionPool(
	output chan<- sip.Message,
	errs chan<- error,
	cancel <-chan struct{},
	msgMapper sip.MessageMapper,
	fields log.Fields,
) ConnectionPool {
	pool := &connectionPool{
		store:     make(map[ConnectionKey]ConnectionHandler),
		msgMapper: msgMapper,

		output: output,
		errs:   errs,
		cancel: cancel,

		done:  make(chan struct{}),
		hMsg:  make(chan sip.Message),
		hErrs: make(chan error),

		hwg: sync.WaitGroup{},
		mu:  sync.RWMutex{},
	}

	pool.fields = fields.
		WithPrefix("transport.ConnectionPool").
		WithFields(log.Fields{
			"connection_pool_ptr": fmt.Sprintf("%p", pool),
		})

	go func() {
		<-pool.cancel
		pool.dispose()
	}()
	go pool.serveHandlers()

	return pool
}

func (pool *connectionPool) String() string {
	if pool == nil {
		return "<nil>"
	}

	return fmt.Sprintf("transport.ConnectionPool<%s>", pool.Fields())
}

func (pool *connectionPool) Fields() log.Fields {
	return pool.fields
}

func (pool *connectionPool) Done() <-chan struct{} {
	return pool.done
}

// Put adds new connection to pool or updates TTL of existing connection
// TTL - 0 - unlimited; 1 - ... - time to live in pool
func (pool *connectionPool) Put(connection Connection, ttl time.Duration) error {
	select {
	case <-pool.cancel:
		return &PoolError{
			fmt.Errorf("connection pool closed"),
			"get connection",
			pool.String(),
		}
	default:
	}

	key := connection.Key()
	if key == "" {
		return &PoolError{
			fmt.Errorf("empty connection key"),
			"put connection",
			pool.String(),
		}
	}

	pool.mu.Lock()
	defer pool.mu.Unlock()
	//todo: add callback if ok here
	return pool.put(key, connection, ttl)
}

func (pool *connectionPool) Get(key ConnectionKey) (Connection, error) {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return pool.getConnection(key)
}

func (pool *connectionPool) Drop(key ConnectionKey) error {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	return pool.drop(key)
}

func (pool *connectionPool) DropAll() error {
	pool.mu.Lock()
	for key := range pool.store {
		if err := pool.drop(key); err != nil {
			pool.Fields().Error("drop connection %s failed: %s", key, err)
		}
	}
	pool.mu.Unlock()

	return nil
}

func (pool *connectionPool) All() []Connection {
	pool.mu.RLock()
	conns := make([]Connection, 0)
	for _, handler := range pool.store {
		conns = append(conns, handler.Connection())
	}
	pool.mu.RUnlock()

	return conns
}

func (pool *connectionPool) Length() int {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return len(pool.store)
}

func (pool *connectionPool) dispose() {
	pool.fields.Debug("disposing...")
	// clean pool
	_ = pool.DropAll()
	pool.fields.Debug("waiting handler done...")
	pool.hwg.Wait()

	// stop serveHandlers goroutine
	close(pool.hMsg)
	close(pool.hErrs)

	close(pool.done)
	pool.fields.Debug("disposed")
}

func (pool *connectionPool) serveHandlers() {
	if log.IsDebugEnabled() {
		pool.fields.Debug("begin serve connection handlers")
		defer pool.fields.Debug("stop serve connection handlers")
	}

	for {
		logger := pool.Fields()

		select {
		case msg, ok := <-pool.hMsg:
			// cancel signal, serveStore exists
			if !ok {
				return
			}
			if msg == nil {
				continue
			}

			select {
			case <-pool.cancel:
				return
			case pool.output <- msg:
				continue
			}
		case err, ok := <-pool.hErrs:
			// cancel signal, serveStore exists
			if !ok {
				return
			}
			if err == nil {
				continue
			}
			// on ConnectionHandleError we should drop handler in some cases
			// all other possible errors ignored because in pool.hErrs should be only ConnectionHandlerErrors
			// so ConnectionPool passes up only Network (when connection falls) and MalformedMessage errors
			var hErr *ConnectionHandlerError
			if !errors.As(err, &hErr) {
				// all other possible errors
				logger.Debug("ignore non connection error: %s", err)
				continue
			}

			pool.mu.RLock()
			handler, gErr := pool.get(hErr.Key)
			pool.mu.RUnlock()
			if gErr != nil {
				// ignore, handler already dropped out
				logger.Debug("ignore error from already dropped out connection %s: %s", hErr.Key, gErr)
				continue
			}

			logger = logger.WithFields(log.Fields{
				"connection_handler": handler.String(),
			})

			if hErr.Expired() {
				// handler expired, drop it from pool and continue without emitting error
				if handler.Expired() {
					// connection expired
					logger.Debug("connection expired, drop it and go further")
					if err := pool.Drop(handler.Key()); err != nil {
						logger.Error(err.Error())
					}
				} else {
					// Due to a race condition, the socket has been updated since this expiry happened.
					// Ignore the expiry since we already have a new socket for this address.
					logger.Debug("ignore spurious connection expiry")
				}

				continue
			} else if hErr.EOF() {
				select {
				case <-pool.cancel:
					return
				default:
				}

				// remote endpoint closed
				if err := pool.Drop(handler.Key()); err != nil {
					logger.Error(err.Error())
				}

				var connErr *ConnectionError
				if errors.As(hErr.Err, &connErr) {
					logger.Debug("conn error:%s", hErr.Err)
				}

				continue
			} else if hErr.Network() {
				if err := pool.Drop(handler.Key()); err != nil {
					logger.Error(err.Error())
				}
			} else {
				// syntax errors, malformed message errors and other
				logger.Debug("connection error: %s; pass the error up", hErr)
			}
			// send initial error
			select {
			case <-pool.cancel:
				return
			case pool.errs <- hErr.Err:
				continue
			}
		}
	}
}

func (pool *connectionPool) put(key ConnectionKey, conn Connection, ttl time.Duration) error {
	if _, err := pool.get(key); err == nil {
		return &PoolError{
			fmt.Errorf("key %s already exists in the pool", key),
			"put connection",
			pool.String(),
		}
	}

	// wrap to handler
	handler := NewConnectionHandler(
		conn,
		ttl,
		pool.hMsg,
		pool.hErrs,
		pool.msgMapper,
		pool.Fields(),
	)

	pool.store[handler.Key()] = handler
	// start serving
	pool.hwg.Add(1)
	go handler.Serve()
	go func() {
		<-handler.Done()
		pool.hwg.Done()
	}()

	return nil
}

func (pool *connectionPool) drop(key ConnectionKey) error {
	// check existence in pool
	handler, err := pool.get(key)
	if err != nil {
		return err
	}

	handler.Cancel()

	// modify store
	delete(pool.store, key)

	return nil
}

func (pool *connectionPool) get(key ConnectionKey) (ConnectionHandler, error) {
	if handler, ok := pool.store[key]; ok {
		return handler, nil
	}

	return nil, &PoolError{
		fmt.Errorf("connection %s not found in the pool", key),
		"get connection",
		pool.String(),
	}
}

func (pool *connectionPool) getConnection(key ConnectionKey) (Connection, error) {
	var conn Connection
	handler, err := pool.get(key)
	if err == nil {
		conn = handler.Connection()
	}
	return conn, err
}

// connectionHandler actually serves associated connection
type connectionHandler struct {
	connection Connection
	msgMapper  sip.MessageMapper

	timer  timing.Timer
	ttl    time.Duration
	expiry time.Time

	output     chan<- sip.Message
	errs       chan<- error
	cancelOnce sync.Once
	canceled   chan struct{}
	done       chan struct{}

	fields log.Fields
}

func NewConnectionHandler(
	conn Connection,
	ttl time.Duration,
	output chan<- sip.Message,
	errs chan<- error,
	msgMapper sip.MessageMapper,
	fields log.Fields,
) ConnectionHandler {
	handler := &connectionHandler{
		connection: conn,
		msgMapper:  msgMapper,

		output:   output,
		errs:     errs,
		canceled: make(chan struct{}),
		done:     make(chan struct{}),

		ttl: ttl,
	}

	handler.fields = fields.
		WithPrefix("transport.ConnectionHandler").
		WithFields(log.Fields{
			"connection_handler_ptr": fmt.Sprintf("%p", handler),
			"connection_ptr":         fmt.Sprintf("%p", conn),
			"connection_key":         conn.Key(),
			"connection_network":     conn.Network(),
		})
	if ttl > 0 {
		handler.expiry = time.Now().Add(ttl)
		handler.timer = timing.NewTimer(ttl)
	} else {
		handler.expiry = time.Time{}
		handler.timer = timing.NewTimer(0)
		if !handler.timer.Stop() {
			<-handler.timer.C()
		}
	}

	if handler.msgMapper == nil {
		handler.msgMapper = func(msg sip.Message) sip.Message {
			return msg
		}
	}

	return handler
}

func (handler *connectionHandler) String() string {
	if handler == nil {
		return "<nil>"
	}

	return fmt.Sprintf("transport.ConnectionHandler<%s>", handler.Fields())
}

func (handler *connectionHandler) Fields() log.Fields {
	return handler.fields
}

func (handler *connectionHandler) Key() ConnectionKey {
	return handler.connection.Key()
}

func (handler *connectionHandler) Connection() Connection {
	return handler.connection
}

func (handler *connectionHandler) Expiry() time.Time {
	return handler.expiry
}

func (handler *connectionHandler) Expired() bool {
	return !handler.Expiry().IsZero() && handler.Expiry().Before(time.Now())
}

// Serve is connection serving loop.
// Waits for the connection to expire, and notifies the pool when it does.
func (handler *connectionHandler) Serve() {
	// start connection serving goroutines
	handler.readConnection()
	close(handler.done)
}

// tcp读取字节流
func (handler *connectionHandler) readStream() {
	msgs := make(chan sip.Message)
	errs := make(chan error)
	pr := parser.NewStreamParser(msgs, errs, handler.Fields())
	saddr := fmt.Sprintf("%v", handler.Connection().RemoteAddr())
	//生产者协程
	go func() {
		defer func() {
			_ = handler.Connection().Close()
			pr.Stop()
			close(msgs)
			close(errs)
		}()
		buf := make([]byte, bufferSize)
		var (
			num int
			err error
		)
		for {
			num, err = handler.Connection().Read(buf)
			if err != nil {
				// broken or closed connection
				// so send error and exit
				handler.handleError(err, saddr)
				return
			}
			data := buf[:num]
			if _, err := pr.Write(data); err != nil {
				handler.handleError(err, saddr)
			}
		}
	}()
	//消费者协程
	for {
		select {
		case <-handler.timer.C():
			if handler.Expiry().IsZero() {
				// handler expiryTime is zero only when TTL = 0 (unlimited handler)
				// so we must not get here with zero expiryTime
				handler.Fields().Fatal("fires expiry timer with ZERO expiryTime")
			}
			// pass up to the pool
			handler.handleError(ExpireError("connection expired"), saddr)
		case msg, ok := <-msgs:
			if !ok {
				return
			}
			handler.handleMessage(msg, saddr)
		case err, ok := <-errs:
			if !ok {
				return
			}
			handler.handleError(err, saddr)
		}
	}
}

// udp读取数据报，由于udp没有连接，所以需要使用raddr做一个parser的pool
// 就是每个地址一个parser
func (handler *connectionHandler) readPacket() {
	//生产者协程
	buf := make([]byte, bufferSize)
	pr := parser.NewPacketParser(log.Fields{})
	//解析
	redirect := func(data []byte, addr net.Addr) {
		// skip empty udp packets
		if len(bytes.Trim(data, "\x00")) == 0 {
			return
		}
		if msg, err := pr.ParseMessage(data); err != nil {
			handler.handleError(err, addr.String())
		} else {
			handler.handleMessage(msg, addr.String())
		}
	}
	var (
		num   int
		err   error
		raddr net.Addr
	)
	for {
		num, raddr, err = handler.Connection().ReadFrom(buf)
		if err != nil {
			handler.fields.Debug("exit")
			return
		}
		cp := make([]byte, num)
		copy(cp, buf[:num])
		go redirect(cp, raddr)
	}
}

func (handler *connectionHandler) readConnection() {
	streamed := handler.Connection().Streamed()
	if streamed {
		handler.readStream()
	} else {
		handler.readPacket()
	}
}

func (handler *connectionHandler) handleMessage(msg sip.Message, raddr string) {
	rhost, rport, _ := net.SplitHostPort(raddr)
	msg.SetDestination(handler.Connection().LocalAddr().String())

	switch msg := msg.(type) {
	case sip.Request:
		// RFC 3261 - 18.2.1
		viaHop, ok := msg.ViaHop()
		if !ok {
			handler.Fields().Warn("ignore message without 'Via' header")
			return
		}

		if rhost != "" && rhost != viaHop.Host {
			viaHop.Params.Add("received", sip.String{Str: rhost})
		}

		// rfc3581
		if viaHop.Params.Has("rport") {
			viaHop.Params.Add("rport", sip.String{Str: rport})
		}

		if !handler.Connection().Streamed() {
			if !viaHop.Params.Has("rport") {
				var port sip.Port
				if viaHop.Port != nil {
					port = *viaHop.Port
				} else {
					port = sip.DefaultPort(handler.Connection().Network())
				}
				raddr = fmt.Sprintf("%s:%d", rhost, port)
			}
		}
		msg.SetTransport(handler.connection.Network())
		msg.SetSource(raddr)
	case sip.Response:
		// Set Remote Address as response source
		msg.SetTransport(handler.connection.Network())
		msg.SetSource(raddr)
	}
	msg = handler.msgMapper(msg.WithFields(log.Fields{
		"connection_key": handler.Connection().Key(),
		"received_at":    time.Now(),
	}))
	// pass up
	handler.output <- msg

	if !handler.Expiry().IsZero() {
		handler.expiry = time.Now().Add(handler.ttl)
		handler.timer.Reset(handler.ttl)
	}
}

func isSyntaxError(err error) bool {
	var perr parser.Error
	if errors.As(err, &perr) && perr.Syntax() {
		return true
	}

	var merr sip.MessageError
	if errors.As(err, &merr) && merr.Broken() {
		return true
	}

	return false
}

func (handler *connectionHandler) handleError(err error, raddr string) {
	if isSyntaxError(err) {
		return
	}

	err = &ConnectionHandlerError{
		err,
		handler.Key(),
		fmt.Sprintf("%p", handler),
		handler.Connection().Network(),
		fmt.Sprintf("%v", handler.Connection().LocalAddr()),
		raddr,
	}

	select {
	case <-handler.canceled:
	case handler.errs <- err:
	}
}

// Cancel simply calls runtime provided cancel function.
func (handler *connectionHandler) Cancel() {
	handler.cancelOnce.Do(func() {
		close(handler.canceled)
		_ = handler.Connection().Close()
		handler.fields.Debug("canceled")
	})
}

func (handler *connectionHandler) Done() <-chan struct{} {
	return handler.done
}
