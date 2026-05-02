// Package registry provides shared registration hooks used by tmetric and
// optional integration packages. It is primarily intended for subpackages such
// as github.com/choveylee/tmetric/registry/pprof.
package registry

import (
	"net/http"
	"sync"
)

// PprofRegistrarFunc registers pprof handlers on the provided mux.
type PprofRegistrarFunc func(*http.ServeMux) error

var (
	mutex sync.Mutex

	pprofRegistrar PprofRegistrarFunc

	pprofWaiters []func()
)

// RegisterPprof registers the function that tmetric uses to attach pprof
// handlers to a metrics HTTP mux. It panics if registrarFunc is nil.
func RegisterPprof(registrarFunc PprofRegistrarFunc) {
	if registrarFunc == nil {
		panic("registry: pprof registrar function must not be nil")
	}

	mutex.Lock()

	pprofRegistrar = registrarFunc

	pendingPprofWaiters := pprofWaiters

	pprofWaiters = nil

	mutex.Unlock()

	for _, pendingPprofWaiter := range pendingPprofWaiters {
		pendingPprofWaiter()
	}
}

// PprofRegistrar returns the currently registered pprof registrar, or nil if no
// registrar has been registered.
func PprofRegistrar() PprofRegistrarFunc {
	mutex.Lock()
	defer mutex.Unlock()

	return pprofRegistrar
}

// WhenPprofAvailable invokes fn immediately when a pprof registrar is already
// available. Otherwise, it arranges for fn to run once pprof support has been
// registered.
func WhenPprofAvailable(fn func()) {
	if fn == nil {
		return
	}

	mutex.Lock()

	if pprofRegistrar != nil {
		mutex.Unlock()
		fn()

		return
	}

	pprofWaiters = append(pprofWaiters, fn)

	mutex.Unlock()
}
