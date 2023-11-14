/**
 * @Author: lidonglin
 * @Description:
 * @File:  metrics.go
 * @Version: 1.0.0
 * @Date: 2022/11/03 15:34
 */

package tmetric

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"time"

	"github.com/choveylee/tcfg"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const MaxLabels = 10

func registerCollector(collector prometheus.Collector) error {
	return prometheus.Register(collector)
}

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

func init() {
	metricEnable := tcfg.DefaultBool(tcfg.LocalKey("METRIC_ENABLE"), false)
	if metricEnable == false {
		return
	}

	metricPath := tcfg.DefaultString(tcfg.LocalKey("METRIC_PATH"), "/metric")
	metricPort := tcfg.DefaultInt(tcfg.LocalKey("METRIC_PORT"), 18089)

	pprofEnable := tcfg.DefaultBool(tcfg.LocalKey("PPROF_ENABLE"), false)

	startMetric(metricPath, metricPort, pprofEnable)
}

func InitMetric(metricPath string, metricPort int, pprofEnable bool) error {
	err := startMetric(metricPath, metricPort, pprofEnable)
	if err != nil {
		return err
	}

	return nil
}

func startMetric(metricPath string, metricPort int, pprofEnable bool) error {
	var metricMux *http.ServeMux

	if pprofEnable == true {
		metricMux = http.DefaultServeMux
	} else {
		metricMux = http.NewServeMux()
	}

	metricMux.Handle(metricPath, promhttp.Handler())

	go func() {
		log.Printf("start metric exporter at %d:%s", metricPort, metricPath)

		err := http.ListenAndServe(fmt.Sprintf(":%d", metricPort), metricMux)
		if err != nil {
			log.Printf("start metric (%s, %d, %v) err (listen and serve %v).",
				metricPath, metricPort, pprofEnable, err)
		}
	}()

	return nil
}
