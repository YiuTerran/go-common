package debugutil

import (
	"log"
	"net/http"
	"net/http/pprof"
)

//专门用来debug的不对外暴露的API

// init disables default handlers registered by importing net/http/pprof.
func init() {
	http.DefaultServeMux = http.NewServeMux()
}

// Handle adds standard pprof handlers to mux.
func Handle(mux *http.ServeMux) {
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
}

// NewServeMux builds a ServeMux and populates it with standard pprof handlers.
func NewServeMux() *http.ServeMux {
	mux := http.NewServeMux()
	Handle(mux)
	return mux
}

// NewServer constructs a server at addr with the standard pprof handlers.
func NewServer(addr string, cb func(mux *http.ServeMux)) *http.Server {
	mux := NewServeMux()
	if cb != nil {
		cb(mux)
	}
	return &http.Server{
		Addr:    addr,
		Handler: mux,
	}
}

// ListenAndServe starts a server at addr with standard pprof handlers.
func ListenAndServe(addr string, cb func(mux *http.ServeMux)) error {
	return NewServer(addr, cb).ListenAndServe()
}

// LaunchHttpServer set a standard pprof server at addr.
// 如果需要自己增加debug method，在cb中添加映射即可
func LaunchHttpServer(addr string, cb func(mux *http.ServeMux)) {
	go func() {
		log.Fatal(ListenAndServe(addr, cb))
	}()
}
