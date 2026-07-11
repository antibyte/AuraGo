package server

import (
	"net/http"
	"net/http/pprof"
)

// registerPProfRoutes wires the standard net/http/pprof handlers into mux.
// Callers must ensure this is only exposed in trusted/debug environments.
func registerPProfRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
}
