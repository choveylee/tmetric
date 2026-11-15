// Package tmetric provides lightweight wrappers around
// [prometheus.CounterVec], [prometheus.GaugeVec], and
// [prometheus.HistogramVec]. Each wrapper validates label-value arity before
// delegating to Prometheus and returns errors for invalid UTF-8 label values or
// uninitialized vectors. Histogram constructors reject the reserved label "le".
// Constructors copy label names, and counter increments must be finite and non-negative.
// Collectors created by this package are
// registered with the default Prometheus registry.
// The HTTP handler reads the default gatherer without implicitly registering scrape instrumentation.
//
// Metrics may be exposed explicitly through [InitMetric] or, when enabled by
// [github.com/choveylee/tcfg], during package initialization through the
// METRIC_ENABLE, METRIC_PATH, METRIC_PORT, and PPROF_ENABLE configuration keys.
// A successful [Shutdown] allows immediate restart; timed-out shutdowns can be retried.
// Shutdown also cancels deferred automatic startup. Complete request reads have a ten-second timeout.
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
