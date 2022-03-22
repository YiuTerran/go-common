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

// handle adds standard pprof handlers to mux.
func handle(mux *http.ServeMux) {
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
}

// newServeMux builds a ServeMux and populates it with standard pprof handlers.
func newServeMux() *http.ServeMux {
	mux := http.NewServeMux()
	handle(mux)
	return mux
}

// newServer constructs a server at addr with the standard pprof handlers.
func newServer(addr string, cb func(mux *http.ServeMux)) *http.Server {
	mux := newServeMux()
	if cb != nil {
		cb(mux)
	}
	return &http.Server{
		Addr:    addr,
		Handler: mux,
	}
}

// listenAndServe starts a server at addr with standard pprof handlers.
func listenAndServe(addr string, cb func(mux *http.ServeMux)) error {
	return newServer(addr, cb).ListenAndServe()
}

// LaunchHttpServer set a standard pprof server at addr.
// 如果需要自己增加debug method，在cb中添加映射即可
func LaunchHttpServer(addr string, cb func(mux *http.ServeMux)) {
	go func() {
		log.Fatal(listenAndServe(addr, cb))
	}()
}
