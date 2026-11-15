package tmetric

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strings"
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
	serverMutex sync.Mutex   // protects active and deferred server startup
	server      *http.Server // non-nil while running or awaiting successful shutdown
	serverDone  <-chan struct{}

	pendingStartupCancel context.CancelFunc
)

// CounterVec wraps [prometheus.CounterVec]. Each method requires exactly one string
// argument per label name supplied to [NewCounterVec].
type CounterVec struct {
	counterVec *prometheus.CounterVec
	labelCount int
}

// Inc increments the counter by one for the label combination labelValues.
//
// It returns an error for incorrect arity, invalid UTF-8, or a nil/uninitialized vector.
func (vector *CounterVec) Inc(labelValues ...string) error {
	if vector == nil || vector.counterVec == nil {
		return errors.New("tmetric: counter vector must be initialized")
	}

	if err := checkLabelCount(len(labelValues), vector.labelCount); err != nil {
		return err
	}

	metric, err := vector.counterVec.GetMetricWithLabelValues(labelValues...)
	if err != nil {
		return fmt.Errorf("tmetric: resolve counter label values: %w", err)
	}

	metric.Inc()

	return nil
}

// Add adds value to the counter for labelValues. The value must be finite and non-negative.
//
// Invalid label values or a nil/uninitialized vector return an error.
func (vector *CounterVec) Add(value float64, labelValues ...string) error {
	if vector == nil || vector.counterVec == nil {
		return errors.New("tmetric: counter vector must be initialized")
	}

	if err := checkLabelCount(len(labelValues), vector.labelCount); err != nil {
		return err
	}

	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return fmt.Errorf("tmetric: counter value must be finite and non-negative; got %g", value)
	}

	metric, err := vector.counterVec.GetMetricWithLabelValues(labelValues...)
	if err != nil {
		return fmt.Errorf("tmetric: resolve counter label values: %w", err)
	}

	metric.Add(value)

	return nil
}

// NewCounterVec builds a [prometheus.CounterVec] with the given metric name, help text,
// and label names, registers it with the default registry, and returns a [CounterVec].
// The slice labelNames is copied and must contain at most [MaxLabels] elements.
func NewCounterVec(name, help string, labelNames []string) (*CounterVec, error) {
	if len(labelNames) > MaxLabels {
		return nil, fmt.Errorf("tmetric: too many label names: got %d, maximum supported is %d", len(labelNames), MaxLabels)
	}

	labelNames = slices.Clone(labelNames)
	counterOptions := prometheus.CounterOpts{
		Name: name,
		Help: help,
	}

	counterVec := prometheus.NewCounterVec(counterOptions, labelNames)
	err := registerCollector(counterVec)
	if err != nil {
		return nil, err
	}

	vector := &CounterVec{counterVec: counterVec, labelCount: len(labelNames)}

	return vector, nil
}

// GaugeVec wraps [prometheus.GaugeVec] and enforces the same label arity rules as [CounterVec].
type GaugeVec struct {
	gaugeVec   *prometheus.GaugeVec
	labelCount int
}

// Set sets the gauge to value for the label combination labelValues.
//
// It returns an error for incorrect arity, invalid UTF-8, or a nil/uninitialized vector.
func (vector *GaugeVec) Set(value float64, labelValues ...string) error {
	if vector == nil || vector.gaugeVec == nil {
		return errors.New("tmetric: gauge vector must be initialized")
	}

	if err := checkLabelCount(len(labelValues), vector.labelCount); err != nil {
		return err
	}

	metric, err := vector.gaugeVec.GetMetricWithLabelValues(labelValues...)
	if err != nil {
		return fmt.Errorf("tmetric: resolve gauge label values: %w", err)
	}

	metric.Set(value)

	return nil
}

// Add adds value to the gauge for the label combination labelValues.
//
// It returns an error for incorrect arity, invalid UTF-8, or a nil/uninitialized vector.
func (vector *GaugeVec) Add(value float64, labelValues ...string) error {
	if vector == nil || vector.gaugeVec == nil {
		return errors.New("tmetric: gauge vector must be initialized")
	}

	if err := checkLabelCount(len(labelValues), vector.labelCount); err != nil {
		return err
	}

	metric, err := vector.gaugeVec.GetMetricWithLabelValues(labelValues...)
	if err != nil {
		return fmt.Errorf("tmetric: resolve gauge label values: %w", err)
	}

	metric.Add(value)

	return nil
}

// NewGaugeVec builds a [prometheus.GaugeVec] with the given metric name, help text, and
// label names, registers it with the default registry, and returns a [GaugeVec].
// The slice labelNames is copied and must contain at most [MaxLabels] elements.
func NewGaugeVec(name, help string, labelNames []string) (*GaugeVec, error) {
	if len(labelNames) > MaxLabels {
		return nil, fmt.Errorf("tmetric: too many label names: got %d, maximum supported is %d", len(labelNames), MaxLabels)
	}

	labelNames = slices.Clone(labelNames)
	gaugeOptions := prometheus.GaugeOpts{
		Name: name,
		Help: help,
	}

	gaugeVec := prometheus.NewGaugeVec(gaugeOptions, labelNames)
	err := registerCollector(gaugeVec)
	if err != nil {
		return nil, err
	}

	vector := &GaugeVec{gaugeVec: gaugeVec, labelCount: len(labelNames)}

	return vector, nil
}

// HistogramVec wraps [prometheus.HistogramVec] with the package default millisecond
// bucket upper bounds for latency-style observations and enforces label arity on each method.
type HistogramVec struct {
	histogramVec *prometheus.HistogramVec
	labelCount   int
}

// Observe records value in the histogram for the label combination labelValues. Buckets use
// millisecond upper bounds; the help string passed to [NewHistogramVec] should document
// the unit of value.
//
// It returns an error for incorrect arity, invalid UTF-8, or a nil/uninitialized vector.
func (vector *HistogramVec) Observe(value float64, labelValues ...string) error {
	if vector == nil || vector.histogramVec == nil {
		return errors.New("tmetric: histogram vector must be initialized")
	}

	if err := checkLabelCount(len(labelValues), vector.labelCount); err != nil {
		return err
	}

	metric, err := vector.histogramVec.GetMetricWithLabelValues(labelValues...)
	if err != nil {
		return fmt.Errorf("tmetric: resolve histogram label values: %w", err)
	}

	metric.Observe(value)

	return nil
}

// NewHistogramVec builds a [prometheus.HistogramVec] with the package default millisecond
// buckets, registers it with the default registry, and returns a [HistogramVec].
// The slice labelNames is copied, must contain at most [MaxLabels] elements,
// and must not include the reserved bucket label "le".
func NewHistogramVec(name, help string, labelNames []string) (*HistogramVec, error) {
	if len(labelNames) > MaxLabels {
		return nil, fmt.Errorf("tmetric: too many label names: got %d, maximum supported is %d", len(labelNames), MaxLabels)
	}

	for _, labelName := range labelNames {
		if labelName == "le" {
			return nil, errors.New("tmetric: histogram label names must not contain 'le'")
		}
	}

	labelNames = slices.Clone(labelNames)
	histogramOptions := prometheus.HistogramOpts{
		Name:    name,
		Help:    help,
		Buckets: defaultLatencyBuckets,
	}

	histogramVec := prometheus.NewHistogramVec(histogramOptions, labelNames)
	err := registerCollector(histogramVec)
	if err != nil {
		return nil, err
	}

	vector := &HistogramVec{histogramVec: histogramVec, labelCount: len(labelNames)}

	return vector, nil
}

// SinceMS returns the elapsed time since startedAt in milliseconds as a float64. The result is
// appropriate for [HistogramVec.Observe] when histogram bucket boundaries are expressed
// in milliseconds.
func SinceMS(startedAt time.Time) float64 {
	return float64(time.Since(startedAt)) / float64(time.Millisecond)
}

// InitMetric listens on all interfaces on metricPort and serves Prometheus metrics at
// metricPath. When pprofEnable is true, it also registers standard-library
// [net/http/pprof] endpoints on that server's [http.ServeMux] under /debug/pprof/,
// provided the optional package at github.com/choveylee/tmetric/registry/pprof has been imported.
//
// metricPath must be a valid literal URL path without dot segments or repeated slashes,
// for example "/metrics". Profiling paths under /debug/pprof are reserved when pprofEnable is true.
//
// At most one metrics HTTP server may run per process. A second call returns an
// error while the first server remains running.
func InitMetric(metricPath string, metricPort int, pprofEnable bool) error {
	return startMetric(metricPath, metricPort, pprofEnable, nil)
}

// Shutdown gracefully stops the metrics HTTP server started by [InitMetric] or during
// package initialization when METRIC_ENABLE is set. It blocks until the server exits or
// ctx is canceled. A successful return permits immediate restart. After a timeout,
// the server remains tracked until a later successful Shutdown. A nil ctx returns an error.
// Pending configuration-driven startup is canceled even if no server is running.
func Shutdown(ctx context.Context) error {
	if ctx == nil {
		return errors.New("tmetric: context must not be nil")
	}

	serverMutex.Lock()
	if pendingStartupCancel != nil {
		pendingStartupCancel()
		pendingStartupCancel = nil
	}

	metricServer := server
	done := serverDone
	serverMutex.Unlock()

	if metricServer == nil {
		return nil
	}

	if err := metricServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("tmetric: shut down metrics HTTP server: %w", err)
	}

	// Serve may not have started when Shutdown ran; wait until its listener and goroutine have exited.
	select {
	case <-done:
	case <-ctx.Done():
		return fmt.Errorf("tmetric: shut down metrics HTTP server: %w", ctx.Err())
	}

	serverMutex.Lock()
	if server == metricServer {
		server = nil
		serverDone = nil
	}

	serverMutex.Unlock()

	return nil
}

// init starts the metrics HTTP server when METRIC_ENABLE is true in tcfg, using
// METRIC_PATH, METRIC_PORT, and PPROF_ENABLE. Startup failures are logged because
// package initialization cannot return errors to the caller.
func init() {
	metricEnable := tcfg.DefaultBool(tcfg.LocalKey("METRIC_ENABLE"), false)
	if !metricEnable {
		return
	}

	metricPath := strings.TrimSpace(tcfg.DefaultString(tcfg.LocalKey("METRIC_PATH"), "/metric"))
	metricPort := tcfg.DefaultInt(tcfg.LocalKey("METRIC_PORT"), 18089)

	pprofEnable := tcfg.DefaultBool(tcfg.LocalKey("PPROF_ENABLE"), false)

	if pprofEnable && registry.PprofRegistrar() == nil {
		log.Printf("tmetric: metrics server startup is deferred until %q is imported", optionalPprofImportPath)

		startupCtx, cancel := context.WithCancel(context.Background())
		serverMutex.Lock()
		pendingStartupCancel = cancel
		serverMutex.Unlock()

		registry.WhenPprofAvailable(func() {
			defer cancel()

			err := startMetric(metricPath, metricPort, pprofEnable, startupCtx)
			if err != nil {
				log.Printf("tmetric: start metrics HTTP server: %v", err)
			}
		})

		return
	}

	if err := startMetric(metricPath, metricPort, pprofEnable, nil); err != nil {
		log.Printf("tmetric: start metrics HTTP server: %v", err)
	}
}

// startMetric binds to metricPort, serves [promhttp.HandlerFor] at metricPath, optionally
// registers pprof routes, and stores the resulting [*http.Server] in the package-level
// server variable. startupCtx gates deferred startup only; it is not the server's request context.
func startMetric(metricPath string, metricPort int, pprofEnable bool, startupCtx context.Context) error {
	if startupCtx != nil && startupCtx.Err() != nil {
		return nil
	}

	if metricPath == "" {
		return errors.New("tmetric: metric path must not be empty")
	}

	if metricPath[0] != '/' {
		return errors.New("tmetric: metric path must begin with '/'")
	}

	if strings.ContainsAny(metricPath, "{}?# \t\r\n") {
		return errors.New("tmetric: metric path must be a literal URL path")
	}

	metricURL, err := url.ParseRequestURI(metricPath)
	if err != nil {
		return fmt.Errorf("tmetric: parse metric path: %w", err)
	}

	cleanPath := path.Clean(metricURL.Path)
	if strings.HasSuffix(metricURL.Path, "/") && cleanPath != "/" {
		cleanPath += "/"
	}

	if metricURL.Path != cleanPath {
		return errors.New("tmetric: metric path must not contain repeated slashes or dot segments")
	}

	if pprofEnable && (cleanPath == "/debug/pprof" || strings.HasPrefix(cleanPath, "/debug/pprof/")) {
		return errors.New("tmetric: metric path must not overlap the pprof endpoints")
	}

	if metricPort < 0 || metricPort > 65535 {
		return errors.New("tmetric: metric port must be between 0 and 65535")
	}

	metricMux := http.NewServeMux()
	// Keep exposition read-only: registering scrape instrumentation here can conflict with application metrics.
	handlerOptions := promhttp.HandlerOpts{ErrorLog: log.Default()}
	metricHandler := promhttp.HandlerFor(prometheus.DefaultGatherer, handlerOptions)

	metricMux.Handle(metricPath, metricHandler)

	if pprofEnable {
		pprofRegistrar := registry.PprofRegistrar()
		if pprofRegistrar == nil {
			return fmt.Errorf("tmetric: pprof support is unavailable; import _ %q to enable it", optionalPprofImportPath)
		}

		if err := pprofRegistrar(metricMux); err != nil {
			return err
		}
	}

	serverMutex.Lock()
	defer serverMutex.Unlock()

	// Recheck under the same lock used by Shutdown so canceled callbacks cannot bind a new listener.
	if startupCtx != nil && startupCtx.Err() != nil {
		return nil
	}

	if server != nil {
		return errors.New("tmetric: metrics HTTP server is already running")
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", metricPort))
	if err != nil {
		return fmt.Errorf("tmetric: listen on metric port %d: %w", metricPort, err)
	}

	metricServer := &http.Server{
		Handler:           metricMux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       10 * time.Second,
		IdleTimeout:       time.Minute,
	}

	done := make(chan struct{})

	server = metricServer
	serverDone = done
	if pendingStartupCancel != nil {
		pendingStartupCancel()
		pendingStartupCancel = nil
	}

	go func() {
		defer close(done)
		log.Printf("tmetric: metrics HTTP server is listening on %s (path=%s, pprof=%t)", listener.Addr().String(), metricPath, pprofEnable)

		err := metricServer.Serve(listener)
		if !errors.Is(err, http.ErrServerClosed) {
			if err != nil {
				log.Printf("tmetric: serve metrics HTTP requests (path=%s, port=%d, pprof=%t): %v", metricPath, metricPort, pprofEnable, err)
			}

			serverMutex.Lock()
			if server == metricServer {
				server = nil
				serverDone = nil
			}

			serverMutex.Unlock()
		}

		// Normal shutdown keeps ownership until Shutdown finishes, so a timeout can be retried.
	}()

	return nil
}

// registerCollector registers collector with the default Prometheus registry.
func registerCollector(collector prometheus.Collector) error {
	if err := prometheus.Register(collector); err != nil {
		return fmt.Errorf("tmetric: register Prometheus collector: %w", err)
	}

	return nil
}

// checkLabelCount verifies that the provided label count matches the vector's construction-time arity.
func checkLabelCount(actualCount, expectedCount int) error {
	if actualCount == expectedCount {
		return nil
	}

	return fmt.Errorf("tmetric: label value count mismatch: got %d, want %d", actualCount, expectedCount)
}
