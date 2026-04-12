/**
 * @Author: lidonglin
 * @Description: Prometheus CounterVec, GaugeVec, and HistogramVec wrappers with label-arity checks, optional HTTP scrape endpoint, Shutdown, and optional pprof routes.
 * @File:  tmetric.go
 * @Version: 1.0.0
 * @Date: 2022/11/03 10:34
 */

// Package tmetric provides wrappers around Prometheus CounterVec, GaugeVec, and HistogramVec
// metrics with validation of label value arity before calling WithLabelValues. Metrics are
// registered with the default Prometheus gatherer. Callers may expose a scrape endpoint with
// InitMetric, or rely on package initialization when tcfg enables it (see METRIC_ENABLE,
// METRIC_PATH, METRIC_PORT, and PPROF_ENABLE).
//
// When the HTTP server is started with pprof enabled, pprof routes are mounted on the same
// http.ServeMux as the Prometheus handler for that server only. Importing net/http/pprof
// still runs its package init, which registers handlers on http.DefaultServeMux; that is
// standard library behavior outside this package's control.
package tmetric

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"sync"
	"time"

	"github.com/choveylee/tcfg"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MaxLabels is the maximum number of label names allowed for NewCounterVec, NewGaugeVec,
// and NewHistogramVec.
const MaxLabels = 10

var (
	mutex  sync.Mutex    // serializes access to server
	server *http.Server // non-nil while the metrics HTTP server is running
)

// registerCollector registers c with the default Prometheus registry.
func registerCollector(c prometheus.Collector) error {
	if err := prometheus.Register(c); err != nil {
		return fmt.Errorf("tmetric: register collector: %w", err)
	}

	return nil
}

// checkLabelCount returns an error if got differs from want, the expected number of label values.
func checkLabelCount(got, want int) error {
	if got == want {
		return nil
	}

	return fmt.Errorf("tmetric: label value count is %d, want %d", got, want)
}

// errLabelNamesTooMany returns an error indicating that got exceeds MaxLabels.
func errLabelNamesTooMany(got int) error {
	return fmt.Errorf("tmetric: label names count %d exceeds maximum %d", got, MaxLabels)
}

// registerPprofHandlers registers the standard pprof endpoints under /debug/pprof/ on mux.
// The metrics listener serves only mux; it does not use http.DefaultServeMux.
//
// Importing package net/http/pprof still registers the same paths on http.DefaultServeMux
// during package initialization.
func registerPprofHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
}

// CounterVec wraps prometheus.CounterVec and enforces that each method receives exactly one
// string per label name declared at construction.
type CounterVec struct {
	counterVec *prometheus.CounterVec

	labelCount int
}

// Inc increments the counter by one for the label values given in lvs.
//
// Inc returns an error if the number of values in lvs does not equal the label count from NewCounterVec.
func (p *CounterVec) Inc(lvs ...string) error {
	if err := checkLabelCount(len(lvs), p.labelCount); err != nil {
		return err
	}

	p.counterVec.WithLabelValues(lvs...).Inc()

	return nil
}

// Add adds v to the counter for the label values in lvs. v must be greater than or equal to zero.
//
// Add returns an error if the label value count is incorrect or if v is negative.
func (p *CounterVec) Add(v float64, lvs ...string) error {
	if err := checkLabelCount(len(lvs), p.labelCount); err != nil {
		return err
	}

	if v < 0 {
		return fmt.Errorf("tmetric: counter Add requires non-negative value (got %g)", v)
	}

	p.counterVec.WithLabelValues(lvs...).Add(v)

	return nil
}

// NewCounterVec creates a counter vector with the given Prometheus name, help text, and
// label names, registers it with the default registry, and returns a CounterVec wrapper.
// The length of labels must not exceed MaxLabels.
func NewCounterVec(name, help string, labels []string) (*CounterVec, error) {
	if len(labels) > MaxLabels {
		return nil, errLabelNamesTooMany(len(labels))
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

// GaugeVec wraps prometheus.GaugeVec and enforces label value arity like CounterVec.
type GaugeVec struct {
	gaugeVec   *prometheus.GaugeVec
	labelCount int
}

// Set sets the gauge to v for the label values in lvs.
//
// Set returns an error if the number of values in lvs does not equal the label count from NewGaugeVec.
func (p *GaugeVec) Set(v float64, lvs ...string) error {
	if err := checkLabelCount(len(lvs), p.labelCount); err != nil {
		return err
	}

	p.gaugeVec.WithLabelValues(lvs...).Set(v)

	return nil
}

// Add adds v to the gauge for the label values in lvs.
//
// Add returns an error if the number of values in lvs does not equal the label count from NewGaugeVec.
func (p *GaugeVec) Add(v float64, lvs ...string) error {
	if err := checkLabelCount(len(lvs), p.labelCount); err != nil {
		return err
	}

	p.gaugeVec.WithLabelValues(lvs...).Add(v)

	return nil
}

// NewGaugeVec creates a gauge vector with the given name, help text, and label names,
// registers it with the default registry, and returns a GaugeVec wrapper.
// The length of labels must not exceed MaxLabels.
func NewGaugeVec(name, help string, labels []string) (*GaugeVec, error) {
	if len(labels) > MaxLabels {
		return nil, errLabelNamesTooMany(len(labels))
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

// HistogramVec wraps prometheus.HistogramVec with default millisecond bucket boundaries
// and enforces label value arity.
type HistogramVec struct {
	histogramVec *prometheus.HistogramVec
	labelCount   int
}

// Observe records v in the histogram for the label values in lvs.
// Bucket boundaries are defined by defaultLatencyBuckets (milliseconds); metric help text should state the unit of v.
//
// Observe returns an error if the number of values in lvs does not equal the label count from NewHistogramVec.
func (p *HistogramVec) Observe(v float64, lvs ...string) error {
	if err := checkLabelCount(len(lvs), p.labelCount); err != nil {
		return err
	}

	p.histogramVec.WithLabelValues(lvs...).Observe(v)

	return nil
}

// NewHistogramVec creates a histogram vector with defaultLatencyBuckets, registers it with
// the default registry, and returns a HistogramVec wrapper.
// The length of labels must not exceed MaxLabels.
func NewHistogramVec(name, help string, labels []string) (*HistogramVec, error) {
	if len(labels) > MaxLabels {
		return nil, errLabelNamesTooMany(len(labels))
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

// SinceMS returns the duration from t until now in milliseconds as a float64 value, suitable
// for passing to [HistogramVec.Observe] when bucket upper bounds are in milliseconds.
func SinceMS(t time.Time) float64 {
	return float64(time.Since(t).Milliseconds())
}

func init() {
	metricEnable := tcfg.DefaultBool(tcfg.LocalKey("METRIC_ENABLE"), false)
	if !metricEnable {
		return
	}

	metricPath := tcfg.DefaultString(tcfg.LocalKey("METRIC_PATH"), "/metric")
	metricPort := tcfg.DefaultInt(tcfg.LocalKey("METRIC_PORT"), 18089)

	pprofEnable := tcfg.DefaultBool(tcfg.LocalKey("PPROF_ENABLE"), false)

	if err := startMetric(metricPath, metricPort, pprofEnable); err != nil {
		log.Printf("start metric err: %v", err)
	}
}

// InitMetric starts an HTTP server listening on all interfaces at metricPort and serves
// Prometheus metrics at metricPath. If pprofEnable is true, standard pprof endpoints are
// also registered on that server's http.ServeMux under /debug/pprof/.
//
// metricPath must be non-empty and must begin with '/', for example "/metrics" or "/metric".
//
// Only one metrics HTTP server may run per process; a second call returns an error while the first remains active.
func InitMetric(metricPath string, metricPort int, pprofEnable bool) error {
	return startMetric(metricPath, metricPort, pprofEnable)
}

// Shutdown shuts down the metrics HTTP server started by InitMetric or by package init when
// METRIC_ENABLE is set. It returns when the server has stopped or ctx is canceled.
// Shutdown returns nil if no server is running.
func Shutdown(ctx context.Context) error {
	mutex.Lock()

	tmpServer := server

	mutex.Unlock()

	if tmpServer == nil {
		return nil
	}

	if err := tmpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("tmetric: metrics HTTP server shutdown: %w", err)
	}
	return nil
}

// startMetric listens on metricPort, attaches promhttp.Handler at metricPath, optionally
// registers pprof handlers, and stores the resulting http.Server in the package variable server.
func startMetric(metricPath string, metricPort int, pprofEnable bool) error {
	if metricPath == "" {
		return fmt.Errorf("tmetric: metricPath must be non-empty")
	}
	if metricPath[0] != '/' {
		return fmt.Errorf("tmetric: metricPath must start with '/'")
	}

	metricMux := http.NewServeMux()
	metricMux.Handle(metricPath, promhttp.Handler())
	if pprofEnable {
		registerPprofHandlers(metricMux)
	}

	mutex.Lock()

	if server != nil {
		mutex.Unlock()

		return fmt.Errorf("tmetric: metrics HTTP server already running")
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", metricPort))
	if err != nil {
		mutex.Unlock()

		return fmt.Errorf("tmetric: listen on :%d: %w", metricPort, err)
	}

	tmpServer := &http.Server{
		Handler:           metricMux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	server = tmpServer

	mutex.Unlock()

	go func() {
		log.Printf("start metric exporter at %s (path %s, pprof %v)", listener.Addr().String(), metricPath, pprofEnable)

		err := tmpServer.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("metric http.Server.Serve (%s, %d, pprof %v): %v", metricPath, metricPort, pprofEnable, err)
		}

		mutex.Lock()

		if server == tmpServer {
			server = nil
		}

		mutex.Unlock()
	}()

	return nil
}
