// Package tmetric provides lightweight wrappers around
// [prometheus.CounterVec], [prometheus.GaugeVec], and
// [prometheus.HistogramVec]. Each wrapper validates label-value arity before
// delegating to Prometheus, avoiding the panic that Prometheus would otherwise
// raise for mismatched arity. Collectors created by this package are
// registered with the default Prometheus registry.
//
// Metrics may be exposed explicitly through [InitMetric] or, when enabled by
// [github.com/choveylee/tcfg], during package initialization through the
// METRIC_ENABLE, METRIC_PATH, METRIC_PORT, and PPROF_ENABLE configuration keys.
//
// Importing the optional package
// github.com/choveylee/tmetric/registry/pprof enables the standard-library
// [net/http/pprof] handlers on the metrics server mux when pprof support is
// requested. Without that optional package, [InitMetric] returns an error when
// pprofEnable is true, and configuration-driven startup is deferred until pprof
// support becomes available. Because the optional package imports
// [net/http/pprof], it also inherits that package's behavior of registering
// handlers on [http.DefaultServeMux].
package tmetric
