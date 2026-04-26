// Package registry provides registration hooks used by tmetric and optional
// integration packages. It is intended primarily for subpackages such as
// github.com/choveylee/tmetric/registry/pprof.
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

// RegisterPprof registers the function used by tmetric to attach pprof handlers
// to a metrics HTTP mux. It panics if registrarFunc is nil.
func RegisterPprof(registrarFunc PprofRegistrarFunc) {
	if registrarFunc == nil {
		panic("registry: pprof registrar must not be nil")
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

// PprofRegistrar returns the currently registered pprof registrar. It returns
// nil if no registrar has been registered.
func PprofRegistrar() PprofRegistrarFunc {
	mutex.Lock()
	defer mutex.Unlock()

	return pprofRegistrar
}

// WhenPprofAvailable invokes fn immediately if a pprof registrar is already
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
