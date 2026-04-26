// Package tmetric provides thin wrappers around [prometheus.CounterVec],
// [prometheus.GaugeVec], and [prometheus.HistogramVec]. Each wrapper validates the
// number of label values supplied at call time before delegating to
// WithLabelValues-style APIs, avoiding the panic that Prometheus would otherwise
// raise for mismatched arity. All collectors created by this package are
// registered with the default Prometheus registry.
//
// Metrics may be exposed explicitly with [InitMetric], or implicitly during package
// initialization when [github.com/choveylee/tcfg] enables METRIC_ENABLE (see also
// METRIC_PATH, METRIC_PORT, and PPROF_ENABLE).
//
// Importing the optional package
// github.com/choveylee/tmetric/registry/pprof enables standard-library
// [net/http/pprof] handlers on the metrics server mux when pprof is requested.
// Without that optional package, explicit pprof startup returns an error and
// package-initialization startup is deferred until pprof support becomes
// available. Because the optional package imports [net/http/pprof], it also
// inherits that package's behavior of registering handlers on
// [http.DefaultServeMux].
package tmetric
