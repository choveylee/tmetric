// Package pprof registers the standard-library [net/http/pprof] handlers with
// tmetric's registry package. Import it for its side effects.
package pprof

import (
	"net/http"
	stdpprof "net/http/pprof"

	"github.com/choveylee/tmetric/registry"
)

func init() {
	pprofRegistrarFunc := func(mux *http.ServeMux) error {
		mux.HandleFunc("GET /debug/pprof/", stdpprof.Index)
		mux.HandleFunc("GET /debug/pprof/cmdline", stdpprof.Cmdline)
		mux.HandleFunc("GET /debug/pprof/profile", stdpprof.Profile)
		mux.HandleFunc("GET /debug/pprof/symbol", stdpprof.Symbol)
		mux.HandleFunc("GET /debug/pprof/trace", stdpprof.Trace)

		return nil
	}

	registry.RegisterPprof(pprofRegistrarFunc)
}
