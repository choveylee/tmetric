// Package tmetric provides wrappers around [prometheus.CounterVec],
// [prometheus.GaugeVec], and [prometheus.HistogramVec]. Before delegating to
// [prometheus.CounterVec.WithLabelValues] and analogous APIs, each wrapper method
// checks that the number of label values matches the arity fixed at construction.
// All collectors are registered with the default Prometheus gatherer.
//
// Callers may expose metrics over HTTP with [InitMetric], or rely on package
// initialization when [github.com/choveylee/tcfg] enables metrics via METRIC_ENABLE
// (see also METRIC_PATH, METRIC_PORT, and PPROF_ENABLE).
//
// If the optional profiling server is enabled, standard pprof routes are mounted on
// the same [http.ServeMux] as the Prometheus handler for that process-local metrics
// server only. Importing [net/http/pprof] still registers handlers on
// [http.DefaultServeMux] during that package's initialization; that is standard
// library behavior and is outside this package's control.
package tmetric
