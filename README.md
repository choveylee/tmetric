# tmetric

`tmetric` provides thin wrappers around Prometheus `CounterVec`, `GaugeVec`, and
`HistogramVec`. The package validates label-value arity before delegating to
Prometheus APIs and can expose metrics through an HTTP endpoint.

## Requirements

- Go **1.25** or later

## Installation

```bash
go get github.com/choveylee/tmetric
```

## Features

- Registers counters, gauges, and histograms with the default Prometheus registry.
- Returns errors for mismatched label-value counts instead of allowing
  `WithLabelValues` to panic.
- Exposes metrics through an optional HTTP server created by `InitMetric`.
- Supports configuration-driven startup through
  [`tcfg`](https://github.com/choveylee/tcfg).
- Supports optional integration with the standard-library `net/http/pprof`
  handlers.

## Quick Start

```go
package main

import (
	"context"

	"github.com/choveylee/tmetric"
)

func main() {
	if err := tmetric.InitMetric("/metrics", 9090, false); err != nil {
		panic(err)
	}
	defer func() { _ = tmetric.Shutdown(context.Background()) }()

	requestsTotal, err := tmetric.NewCounterVec(
		"http_requests_total",
		"Total number of HTTP requests.",
		[]string{"method"},
	)
	if err != nil {
		panic(err)
	}

	_ = requestsTotal.Inc("GET")
}
```

## Configuration

When `METRIC_ENABLE` is true, package initialization attempts to start the
metrics HTTP server automatically by reading the following `tcfg` keys:

| Key | Default | Description |
|-----|---------|-------------|
| `METRIC_ENABLE` | `false` | Enables automatic startup of the metrics HTTP server. |
| `METRIC_PATH` | `/metric` | HTTP path used to serve Prometheus metrics. |
| `METRIC_PORT` | `18089` | TCP port used by the metrics HTTP server. |
| `PPROF_ENABLE` | `false` | Enables pprof integration on the same HTTP mux as the metrics handler. |

## Optional pprof Integration

By default, importing `tmetric` does not register profiling handlers on
`http.DefaultServeMux`.

To enable standard-library pprof support, import the optional package alongside
`tmetric`:

```go
import (
	"github.com/choveylee/tmetric"
	_ "github.com/choveylee/tmetric/registry/pprof"
)
```

Importing `github.com/choveylee/tmetric/registry/pprof` also imports
`net/http/pprof`. As defined by the Go standard library, that package registers
its handlers on `http.DefaultServeMux`.

If `InitMetric` is called with `pprofEnable=true` before the optional package is
imported, `InitMetric` returns an error. During configuration-driven startup,
package initialization defers metrics server startup until pprof support becomes
available. If the optional package is never imported, the server is not started
automatically.

## Histograms

`NewHistogramVec` uses millisecond-scale buckets (see
`defaultLatencyBuckets` in the source). `SinceMS` reports elapsed time in
fractional milliseconds as a `float64`, which is suitable for
`HistogramVec.Observe`.

## Testing

```bash
go test ./...
go vet ./...
```
