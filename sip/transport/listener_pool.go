package transport

import (
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/YiuTerran/go-common/base/log"
	"net"
	"strings"
	"sync"
)

type ListenerKey string

func (key ListenerKey) String() string {
	return string(key)
}

type ListenerPool interface {
	Done() <-chan struct{}
	String() string
	Put(key ListenerKey, listener net.Listener) error
	Get(key ListenerKey) (net.Listener, error)
	All() []net.Listener
	Drop(key ListenerKey) error
	DropAll() error
	Length() int
	Fields() log.Fields
}

type ListenerHandler interface {
	Cancel()
	Done() <-chan struct{}
	String() string
	Key() ListenerKey
	Listener() net.Listener
	Serve()
	Fields() log.Fields
}

type listenerPool struct {
	hwg   sync.WaitGroup
	mu    sync.RWMutex
	store map[ListenerKey]ListenerHandler

	output chan<- Connection
	errs   chan<- error
	cancel <-chan struct{}

	done   chan struct{}
	hConns chan Connection
	hErrs  chan error

	fields log.Fields
}

func NewListenerPool(
	output chan<- Connection,
	errs chan<- error,
	cancel <-chan struct{},
	fields log.Fields,
) ListenerPool {
	pool := &listenerPool{
		store: make(map[ListenerKey]ListenerHandler),

		output: output,
		errs:   errs,
		cancel: cancel,

		done:   make(chan struct{}),
		hConns: make(chan Connection),
		hErrs:  make(chan error),
	}
	pool.fields = fields.
		WithPrefix("transport.ListenerPool").
		WithFields(log.Fields{
			"listener_pool_ptr": fmt.Sprintf("%p", pool),
		})

	go func() {
		<-pool.cancel
		pool.dispose()
	}()
	go pool.serveHandlers()

	return pool
}

func (pool *listenerPool) String() string {
	if pool == nil {
		return "<nil>"
	}

	return fmt.Sprintf("transport.ListenerPool<%s>", pool.Fields())
}

func (pool *listenerPool) Fields() log.Fields {
	return pool.fields
}

// Done returns channel that resolves when pool gracefully completes it work.
func (pool *listenerPool) Done() <-chan struct{} {
	return pool.done
}

func (pool *listenerPool) Put(key ListenerKey, listener net.Listener) error {
	select {
	case <-pool.cancel:
		return &PoolError{
			fmt.Errorf("listener pool closed"),
			"put listener",
			pool.String(),
		}
	default:
	}
	if key == "" {
		return &PoolError{
			fmt.Errorf("empty listener key"),
			"put listener",
			pool.String(),
		}
	}

	pool.mu.Lock()
	defer pool.mu.Unlock()

	return pool.put(key, listener)
}

func (pool *listenerPool) Get(key ListenerKey) (net.Listener, error) {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return pool.getListener(key)
}

func (pool *listenerPool) Drop(key ListenerKey) error {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	return pool.drop(key)
}

func (pool *listenerPool) DropAll() error {
	pool.mu.Lock()
	for key := range pool.store {
		if err := pool.drop(key); err != nil {
			pool.Fields().Error("drop listener %s failed: %s", key, err)
		}
	}
	pool.mu.Unlock()

	return nil
}

func (pool *listenerPool) All() []net.Listener {
	pool.mu.RLock()
	listeners := make([]net.Listener, 0)
	for _, handler := range pool.store {
		listeners = append(listeners, handler.Listener())
	}
	pool.mu.RUnlock()

	return listeners
}

func (pool *listenerPool) Length() int {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return len(pool.store)
}

func (pool *listenerPool) dispose() {
	// clean pool
	_ = pool.DropAll()
	pool.hwg.Wait()

	// stop serveHandlers goroutine
	close(pool.hConns)
	close(pool.hErrs)

	close(pool.done)
}

func (pool *listenerPool) serveHandlers() {
	pool.Fields().Debug("start serve listener handlers")
	defer pool.Fields().Debug("stop serve listener handlers")

	for {
		logger := pool.Fields()

		select {
		case conn, ok := <-pool.hConns:
			if !ok {
				return
			}
			if conn == nil {
				continue
			}

			select {
			case <-pool.cancel:
				return
			case pool.output <- conn:
			}
		case err, ok := <-pool.hErrs:
			if !ok {
				return
			}
			if err == nil {
				continue
			}

			var lErr *ListenerHandlerError
			if errors.As(err, &lErr) {
				pool.mu.RLock()
				handler, gErr := pool.get(lErr.Key)
				pool.mu.RUnlock()
				if gErr == nil {
					logger = logger.WithFields(handler.Fields())

					if lErr.Network() {
						// listener broken or closed, should be dropped
						logger.Debug("listener network error: %s; drop it and go further", lErr)
						if err := pool.Drop(handler.Key()); err != nil {
							logger.Error("fail to drop network error listener from pool:%s", err.Error())
						}
					} else {
						// other
						logger.Debug("listener error: %s; pass the error up", lErr)
					}
				} else {
					// ignore, handler already dropped out
					logger.Debug("ignore error from already dropped out listener %s: %s", lErr.Key, lErr)
					continue
				}
			} else {
				// all other possible errors
				logger.Debug("ignore non listener error: %s", err)
				continue
			}

			select {
			case <-pool.cancel:
				return
			case pool.errs <- err:
			}
		}
	}
}

func (pool *listenerPool) put(key ListenerKey, listener net.Listener) error {
	if _, err := pool.get(key); err == nil {
		return &PoolError{
			fmt.Errorf("key %s already exists in the pool", key),
			"put listener",
			pool.String(),
		}
	}

	// wrap to handler
	handler := NewListenerHandler(key, listener, pool.hConns, pool.hErrs, pool.Fields())
	// update store
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

func (pool *listenerPool) drop(key ListenerKey) error {
	// check existence in pool
	handler, err := pool.get(key)
	if err != nil {
		return err
	}

	handler.Cancel()

	pool.Fields().WithFields(handler.Fields()).Debug("drop listener from the pool")

	// modify store
	delete(pool.store, key)

	return nil
}

func (pool *listenerPool) get(key ListenerKey) (ListenerHandler, error) {
	if handler, ok := pool.store[key]; ok {
		return handler, nil
	}

	return nil, &PoolError{
		fmt.Errorf("listenr %s not found in the pool", key),
		"get listener",
		pool.String(),
	}
}

func (pool *listenerPool) getListener(key ListenerKey) (net.Listener, error) {
	if handler, err := pool.get(key); err == nil {
		return handler.Listener(), nil
	} else {
		return nil, err
	}
}

type listenerHandler struct {
	key      ListenerKey
	listener net.Listener

	output chan<- Connection
	errs   chan<- error

	cancelOnce sync.Once
	canceled   chan struct{}
	done       chan struct{}

	fields log.Fields
}

func NewListenerHandler(
	key ListenerKey,
	listener net.Listener,
	output chan<- Connection,
	errs chan<- error,
	fields log.Fields,
) ListenerHandler {
	handler := &listenerHandler{
		key:      key,
		listener: listener,

		output: output,
		errs:   errs,

		canceled: make(chan struct{}),
		done:     make(chan struct{}),
	}

	handler.fields = fields.
		WithPrefix("transport.ListenerHandler").
		WithFields(log.Fields{
			"listener_handler_ptr": fmt.Sprintf("%p", handler),
			"listener_ptr":         fmt.Sprintf("%p", listener),
			"listener_key":         key,
		})

	return handler
}

func (handler *listenerHandler) String() string {
	if handler == nil {
		return "<nil>"
	}

	return fmt.Sprintf("transport.ListenerHandler<%s>", handler.Fields())
}

func (handler *listenerHandler) Fields() log.Fields {
	return handler.fields
}

func (handler *listenerHandler) Key() ListenerKey {
	return handler.key
}

func (handler *listenerHandler) Listener() net.Listener {
	return handler.listener
}

func (handler *listenerHandler) Serve() {
	defer close(handler.done)
	if log.IsDebugEnabled() {
		handler.Fields().Debug("begin serve listener")
		defer handler.Fields().Debug("stop serve listener")
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go handler.acceptConnections(wg)

	wg.Wait()
}

func (handler *listenerHandler) acceptConnections(wg *sync.WaitGroup) {
	defer func() {
		_ = handler.Listener().Close()
		wg.Done()
	}()
	if log.IsDebugEnabled() {
		handler.Fields().Debug("begin accept connections")
		defer handler.Fields().Debug("stop accept connections")
	}

	for {
		// wait for the new connection
		baseConn, err := handler.Listener().Accept()
		if err != nil {
			// broken or closed listener
			// pass up error and exit
			err = &ListenerHandlerError{
				err,
				handler.Key(),
				fmt.Sprintf("%p", handler),
				listenerNetwork(handler.Listener()),
				handler.Listener().Addr().String(),
			}

			select {
			case <-handler.canceled:
			case handler.errs <- err:
			}
			return
		}

		var network string
		switch baseConn.(type) {
		case *tls.Conn:
			network = "tls"
		default:
			network = strings.ToLower(baseConn.RemoteAddr().Network())
		}

		key := ConnectionKey(network + ":" + baseConn.RemoteAddr().String())
		handler.output <- NewConnection(baseConn, key, network, handler.Fields())
	}
}

// Cancel stops serving.
// blocked until Serve completes
func (handler *listenerHandler) Cancel() {
	handler.cancelOnce.Do(func() {
		close(handler.canceled)
		_ = handler.Listener().Close()

		handler.Fields().Debug("listener handler canceled")
	})
}

// Done returns channel that resolves when handler gracefully completes it work.
func (handler *listenerHandler) Done() <-chan struct{} {
	return handler.done
}

func listenerNetwork(ls net.Listener) string {
	if val, ok := ls.(interface{ Network() string }); ok {
		return val.Network()
	}

	switch ls.(type) {
	case *net.TCPListener:
		return "tcp"
	case *net.UnixListener:
		return "unix"
	default:
		return ""
	}
}
