package tmetric

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/choveylee/tcfg"
	"github.com/choveylee/tmetric/registry"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MaxLabels is the inclusive upper bound on the number of label names permitted when
// constructing vectors with [NewCounterVec], [NewGaugeVec], or [NewHistogramVec].
const MaxLabels = 10

const optionalPprofImportPath = "github.com/choveylee/tmetric/registry/pprof"

var (
	mutex  sync.Mutex   // protects server
	server *http.Server // non-nil while the metrics HTTP server is running
)

// registerCollector registers c with the default Prometheus registry.
func registerCollector(c prometheus.Collector) error {
	if err := prometheus.Register(c); err != nil {
		return fmt.Errorf("tmetric: failed to register Prometheus collector: %w", err)
	}

	return nil
}

// checkLabelCount returns nil when got equals want. Otherwise it returns an error
// reporting that the number of label values (got) does not match the expected count (want).
func checkLabelCount(got, want int) error {
	if got == want {
		return nil
	}

	return fmt.Errorf("tmetric: label value count mismatch: got %d, want %d", got, want)
}

func registerPprofHandlers(mux *http.ServeMux) error {
	pprofRegistrar := registry.PprofRegistrar()

	if pprofRegistrar == nil {
		return fmt.Errorf("tmetric: pprof support is unavailable; import _ %q to enable it", optionalPprofImportPath)
	}

	return pprofRegistrar(mux)
}

// CounterVec wraps [prometheus.CounterVec]. Each method requires exactly one string
// argument per label name supplied to [NewCounterVec].
type CounterVec struct {
	counterVec *prometheus.CounterVec

	labelCount int
}

// Inc increments the counter by one for the label combination lvs.
//
// It returns an error if len(lvs) does not equal the label arity fixed at construction.
func (p *CounterVec) Inc(lvs ...string) error {
	if err := checkLabelCount(len(lvs), p.labelCount); err != nil {
		return err
	}

	p.counterVec.WithLabelValues(lvs...).Inc()

	return nil
}

// Add adds v to the counter for the label combination lvs. The value v must be
// non-negative.
//
// It returns an error if the number of label values is incorrect or if v is negative.
func (p *CounterVec) Add(v float64, lvs ...string) error {
	if err := checkLabelCount(len(lvs), p.labelCount); err != nil {
		return err
	}

	if v < 0 {
		return fmt.Errorf("tmetric: counter value must be non-negative; got %g", v)
	}

	p.counterVec.WithLabelValues(lvs...).Add(v)

	return nil
}

// NewCounterVec builds a [prometheus.CounterVec] with the given metric name, help text,
// and label names, registers it with the default registry, and returns a [CounterVec].
// The slice labels must contain at most [MaxLabels] elements.
func NewCounterVec(name, help string, labels []string) (*CounterVec, error) {
	if len(labels) > MaxLabels {
		return nil, fmt.Errorf("tmetric: too many label names: got %d, maximum supported is %d", len(labels), MaxLabels)
	}

	counterVec := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: name,
			Help: help,
		},
		labels,
	)

	err := registerCollector(counterVec)
	if err != nil {
		return nil, err
	}

	return &CounterVec{counterVec: counterVec, labelCount: len(labels)}, nil
}

// GaugeVec wraps [prometheus.GaugeVec] and enforces the same label arity rules as [CounterVec].
type GaugeVec struct {
	gaugeVec   *prometheus.GaugeVec
	labelCount int
}

// Set sets the gauge to v for the label combination lvs.
//
// It returns an error if len(lvs) does not equal the label arity fixed at construction.
func (p *GaugeVec) Set(v float64, lvs ...string) error {
	if err := checkLabelCount(len(lvs), p.labelCount); err != nil {
		return err
	}

	p.gaugeVec.WithLabelValues(lvs...).Set(v)

	return nil
}

// Add adds v to the gauge for the label combination lvs.
//
// It returns an error if len(lvs) does not equal the label arity fixed at construction.
func (p *GaugeVec) Add(v float64, lvs ...string) error {
	if err := checkLabelCount(len(lvs), p.labelCount); err != nil {
		return err
	}

	p.gaugeVec.WithLabelValues(lvs...).Add(v)

	return nil
}

// NewGaugeVec builds a [prometheus.GaugeVec] with the given metric name, help text, and
// label names, registers it with the default registry, and returns a [GaugeVec].
// The slice labels must contain at most [MaxLabels] elements.
func NewGaugeVec(name, help string, labels []string) (*GaugeVec, error) {
	if len(labels) > MaxLabels {
		return nil, fmt.Errorf("tmetric: too many label names: got %d, maximum supported is %d", len(labels), MaxLabels)
	}

	gaugeVec := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: name,
			Help: help,
		},
		labels,
	)

	err := registerCollector(gaugeVec)
	if err != nil {
		return nil, err
	}

	return &GaugeVec{gaugeVec: gaugeVec, labelCount: len(labels)}, nil
}

// HistogramVec wraps [prometheus.HistogramVec] with the package default millisecond
// bucket upper bounds for latency-style observations and enforces label arity on each method.
type HistogramVec struct {
	histogramVec *prometheus.HistogramVec
	labelCount   int
}

// Observe records v in the histogram for the label combination lvs. Buckets use
// millisecond upper bounds; the help string passed to [NewHistogramVec] should document
// the unit of v.
//
// It returns an error if len(lvs) does not equal the label arity fixed at construction.
func (p *HistogramVec) Observe(v float64, lvs ...string) error {
	if err := checkLabelCount(len(lvs), p.labelCount); err != nil {
		return err
	}

	p.histogramVec.WithLabelValues(lvs...).Observe(v)

	return nil
}

// NewHistogramVec builds a [prometheus.HistogramVec] with the package default millisecond
// buckets, registers it with the default registry, and returns a [HistogramVec].
// The slice labels must contain at most [MaxLabels] elements.
func NewHistogramVec(name, help string, labels []string) (*HistogramVec, error) {
	if len(labels) > MaxLabels {
		return nil, fmt.Errorf("tmetric: too many label names: got %d, maximum supported is %d", len(labels), MaxLabels)
	}

	histogramVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    name,
			Help:    help,
			Buckets: defaultLatencyBuckets,
		},
		labels,
	)

	err := registerCollector(histogramVec)
	if err != nil {
		return nil, err
	}

	return &HistogramVec{histogramVec: histogramVec, labelCount: len(labels)}, nil
}

// SinceMS returns the elapsed time since t in milliseconds as a float64. The result is
// appropriate for [HistogramVec.Observe] when histogram bucket boundaries are expressed
// in milliseconds.
func SinceMS(t time.Time) float64 {
	return float64(time.Since(t)) / float64(time.Millisecond)
}

// init starts the metrics HTTP server when METRIC_ENABLE is true in tcfg, using
// METRIC_PATH, METRIC_PORT, and PPROF_ENABLE. Startup failures are logged because
// package initialization cannot return errors to the caller.
func init() {
	metricEnable := tcfg.DefaultBool(tcfg.LocalKey("METRIC_ENABLE"), false)
	if !metricEnable {
		return
	}

	metricPath := tcfg.DefaultString(tcfg.LocalKey("METRIC_PATH"), "/metric")
	metricPort := tcfg.DefaultInt(tcfg.LocalKey("METRIC_PORT"), 18089)

	pprofEnable := tcfg.DefaultBool(tcfg.LocalKey("PPROF_ENABLE"), false)

	if pprofEnable && registry.PprofRegistrar() == nil {
		log.Printf("tmetric: metrics server startup is deferred until %q is imported", optionalPprofImportPath)

		registry.WhenPprofAvailable(func() {
			err := startMetric(metricPath, metricPort, pprofEnable)
			if err != nil {
				log.Printf("tmetric: failed to start metrics HTTP server: %v", err)
			}
		})

		return
	}

	if err := startMetric(metricPath, metricPort, pprofEnable); err != nil {
		log.Printf("tmetric: failed to start metrics HTTP server: %v", err)
	}
}

// InitMetric listens on all interfaces on metricPort and serves Prometheus metrics at
// metricPath. When pprofEnable is true, it also registers standard-library
// [net/http/pprof] endpoints on that server's [http.ServeMux] under /debug/pprof/,
// provided the optional package at github.com/choveylee/tmetric/registry/pprof has been imported.
//
// metricPath must be non-empty and must begin with '/', for example "/metrics" or
// "/metric".
//
// At most one metrics HTTP server may run per process. A second call returns an
// error while the first server remains running.
func InitMetric(metricPath string, metricPort int, pprofEnable bool) error {
	return startMetric(metricPath, metricPort, pprofEnable)
}

// Shutdown gracefully stops the metrics HTTP server started by [InitMetric] or during
// package initialization when METRIC_ENABLE is set. It blocks until the server exits or
// ctx is canceled. If no server is running, Shutdown returns nil.
func Shutdown(ctx context.Context) error {
	mutex.Lock()

	tmpServer := server

	mutex.Unlock()

	if tmpServer == nil {
		return nil
	}

	if err := tmpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("tmetric: failed to shut down metrics HTTP server: %w", err)
	}
	return nil
}

// startMetric binds to metricPort, serves [promhttp.Handler] at metricPath, optionally
// registers pprof routes, and stores the resulting [*http.Server] in the package-level
// server variable.
func startMetric(metricPath string, metricPort int, pprofEnable bool) error {
	if metricPath == "" {
		return fmt.Errorf("tmetric: metric path must not be empty")
	}
	if metricPath[0] != '/' {
		return fmt.Errorf("tmetric: metric path must begin with '/'")
	}

	metricMux := http.NewServeMux()

	metricMux.Handle(metricPath, promhttp.Handler())

	if pprofEnable {
		err := registerPprofHandlers(metricMux)
		if err != nil {
			return err
		}
	}

	mutex.Lock()

	if server != nil {
		mutex.Unlock()

		return fmt.Errorf("tmetric: metrics HTTP server is already running")
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", metricPort))
	if err != nil {
		mutex.Unlock()

		return fmt.Errorf("tmetric: failed to listen on :%d: %w", metricPort, err)
	}

	tmpServer := &http.Server{
		Handler:           metricMux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	server = tmpServer

	mutex.Unlock()

	go func() {
		log.Printf("tmetric: metrics HTTP server is listening on %s (path=%s, pprof=%t)", listener.Addr().String(), metricPath, pprofEnable)

		err := tmpServer.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("tmetric: metrics HTTP server terminated unexpectedly (path=%s, port=%d, pprof=%t): %v", metricPath, metricPort, pprofEnable, err)
		}

		mutex.Lock()

		if server == tmpServer {
			server = nil
		}

		mutex.Unlock()
	}()

	return nil
}
