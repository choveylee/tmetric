/**
 * @Author: lidonglin
 * @Description:
 * @File:  metrics.go
 * @Version: 1.0.0
 * @Date: 2022/11/03 15:34
 */

package tmetric

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"time"

	"github.com/choveylee/tcfg"
	"github.com/choveylee/tlog"
	"github.com/prometheus/client_golang/prometheus"
	otelprometheus "go.opentelemetry.io/otel/exporters/metric/prometheus"
	export "go.opentelemetry.io/otel/sdk/export/metric"
	"go.opentelemetry.io/otel/sdk/metric/aggregator/histogram"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	selector "go.opentelemetry.io/otel/sdk/metric/selector/simple"
)

const MaxLabels = 10

type CounterVec struct {
	counterVec *prometheus.CounterVec
}

func (p *CounterVec) Inc(lvs ...string) {
	p.counterVec.WithLabelValues(lvs...).Inc()
}

func (p *CounterVec) Add(v float64, lvs ...string) error {
	if v < 0 {
		return fmt.Errorf("value should not be negative")
	}

	p.counterVec.WithLabelValues(lvs...).Add(v)

	return nil
}

func NewCounterVec(name, help string, labels []string) (*CounterVec, error) {
	if len(labels) > MaxLabels {
		return nil, fmt.Errorf("too many labels")
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

	return &CounterVec{counterVec: counterVec}, nil
}

type GaugeVec struct {
	gaugeVec *prometheus.GaugeVec
}

func (p *GaugeVec) Set(v float64, lvs ...string) {
	p.gaugeVec.WithLabelValues(lvs...).Set(v)
}

func (p *GaugeVec) Add(v float64, lvs ...string) {
	p.gaugeVec.WithLabelValues(lvs...).Add(v)
}

func NewGaugeVec(name, help string, labels []string) (*GaugeVec, error) {
	if len(labels) > MaxLabels {
		return nil, fmt.Errorf("too many labels")
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

	return &GaugeVec{gaugeVec: gaugeVec}, nil
}

type HistogramVec struct {
	histogramVec *prometheus.HistogramVec
}

func (p *HistogramVec) Observe(v float64, lvs ...string) {
	p.histogramVec.WithLabelValues(lvs...).Observe(v)
}

func NewHistogramVec(name, help string, labels []string) (*HistogramVec, error) {
	if len(labels) > MaxLabels {
		return nil, fmt.Errorf("too many labels")
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

	return &HistogramVec{histogramVec: histogramVec}, nil
}

func SinceMS(t time.Time) float64 {
	return float64(time.Now().Sub(t).Milliseconds())
}

func registerCollector(collector prometheus.Collector) error {
	return prometheus.Register(collector)
}

var defaultLatencyBuckets = []float64{
	1.0, 2.0, 3.0, 4.0, 5.0,
	6.0, 8.0, 10.0, 13.0, 16.0,
	20.0, 25.0, 30.0, 40.0, 50.0,
	65.0, 80.0, 100.0, 130.0, 160.0,
	200.0, 250.0, 300.0, 400.0, 500.0,
	650.0, 800.0, 1000.0, 2000.0, 5000.0,
	10000.0, 20000.0, 50000.0, 100000.0,
}

func withMetricHandler() (http.Handler, error) {
	registry, _ := prometheus.DefaultRegisterer.(*prometheus.Registry)
	config := otelprometheus.Config{}

	controller := controller.New(
		processor.New(
			selector.NewWithHistogramDistribution(
				histogram.WithExplicitBoundaries(config.DefaultHistogramBoundaries),
			),
			export.CumulativeExportKindSelector(),
			processor.WithMemory(true),
		),
	)

	exporter, err := otelprometheus.New(
		otelprometheus.Config{Registry: registry},
		controller,
	)
	if err != nil {
		return nil, fmt.Errorf("install prometheus pipeline: %v", err)
	}

	return exporter, nil
}

func init() {
	metricEnable := tcfg.DefaultBool(tcfg.LocalKey("METRIC_ENABLE"), false)

	if metricEnable == false {
		return
	}

	metricPath := tcfg.DefaultString(tcfg.LocalKey("METRIC_PATH"), "/metric")
	metricPort := tcfg.DefaultInt(tcfg.LocalKey("METRIC_PORT"), 18089)

	handler, err := withMetricHandler()
	if err != nil {
		tlog.E(context.Background()).Err(err).Msgf("init metrics failed")
		return
	}

	var metricMux *http.ServeMux

	pprofEnable := tcfg.DefaultBool(tcfg.LocalKey("PPROF_ENABLE"), false)

	if pprofEnable == true {
		metricMux = http.DefaultServeMux
	} else {
		metricMux = http.NewServeMux()
	}

	metricMux.Handle(metricPath, handler)

	go func() {
		tlog.I(context.Background()).Msgf("starting exporter at %d", metricPort)

		err := http.ListenAndServe(fmt.Sprintf(":%d", metricPort), metricMux)
		if err != nil {
			tlog.E(context.Background()).Err(err).Msgf("start exporter at %d failed", metricPort)
		}
	}()
}
