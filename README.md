# tmetric

Thin wrappers around [Prometheus](https://prometheus.io/) `CounterVec`, `GaugeVec`, and `HistogramVec` with runtime checks on label value arity, plus an optional HTTP scrape endpoint.

## Requirements

- Go **1.25** or later (see [`go.mod`](go.mod))

## Installation

```bash
go get github.com/choveylee/tmetric
```

## Overview

- **Constructors** — `NewCounterVec`, `NewGaugeVec`, and `NewHistogramVec` register metrics with the default Prometheus registry. Each accepts at most **`MaxLabels` (10)** label names.
- **Label arity** — Methods such as `Inc`, `Add`, `Set`, and `Observe` return an error if the number of label values does not match the constructor, avoiding panics from `WithLabelValues`.
- **HTTP endpoint** — `InitMetric` listens on `:{port}` on all interfaces and serves `promhttp.Handler` at `metricPath` (must be non-empty and start with `/`, e.g. `/metrics`). At most **one** such server may run per process; use **`Shutdown`** for graceful termination.
- **Optional startup via tcfg** — When `METRIC_ENABLE` is true ([tcfg](https://github.com/choveylee/tcfg)), package `init` starts the server using `METRIC_PATH`, `METRIC_PORT`, and `PPROF_ENABLE`.

## Configuration (tcfg keys)

| Key | Default | Description |
|-----|---------|-------------|
| `METRIC_ENABLE` | `false` | If true, `init` starts the metrics HTTP server |
| `METRIC_PATH` | `/metric` | URL path for Prometheus scraping |
| `METRIC_PORT` | `18089` | TCP port (`:{port}` on all interfaces) |
| `PPROF_ENABLE` | `false` | If true, registers `net/http/pprof` handlers on the **same** `http.ServeMux` as the metrics handler, under `/debug/pprof/` |

Importing `net/http/pprof` still runs its package `init`, which registers handlers on `http.DefaultServeMux`; that behavior is defined by the Go standard library.

## Example

```go
import (
	"context"

	"github.com/choveylee/tmetric"
)

func Example() error {
	if err := tmetric.InitMetric("/metrics", 9090, false); err != nil {
		return err
	}
	defer func() { _ = tmetric.Shutdown(context.Background()) }()

	cv, err := tmetric.NewCounterVec("http_requests_total", "Total HTTP requests", []string{"method"})
	if err != nil {
		return err
	}
	return cv.Inc("GET")
}
```

## Histograms

`NewHistogramVec` uses millisecond-scale buckets (`defaultLatencyBuckets` in the source). Document the unit of observed values in metric names or help text; `SinceMS` reports elapsed time in milliseconds as a `float64` for use with `Observe`.

## Tests

```bash
go test ./...
```

If `METRIC_ENABLE` is true when the package loads, `init` starts the HTTP server and tests that assume no server may need adjustment.
