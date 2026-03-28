package core

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

// NewCounter creates and registers a Prometheus counter with the standard KORS naming convention:
// kors_{service}_{name}_total
// Labels are passed as label names; values are set at observation time via .WithLabelValues().
func NewCounter(reg prometheus.Registerer, service, name, help string, labels []string) *prometheus.CounterVec {
	c := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: fmt.Sprintf("kors_%s_%s_total", service, name),
		Help: help,
	}, labels)
	reg.MustRegister(c)
	return c
}

// NewHistogram creates and registers a Prometheus histogram with the standard KORS naming convention:
// kors_{service}_{name}
func NewHistogram(reg prometheus.Registerer, service, name, help string, labels []string, buckets []float64) *prometheus.HistogramVec {
	if len(buckets) == 0 {
		buckets = prometheus.DefBuckets
	}
	h := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    fmt.Sprintf("kors_%s_%s", service, name),
		Help:    help,
		Buckets: buckets,
	}, labels)
	reg.MustRegister(h)
	return h
}

// NewGauge creates and registers a Prometheus gauge with the standard KORS naming convention:
// kors_{service}_{name}
func NewGauge(reg prometheus.Registerer, service, name, help string) prometheus.Gauge {
	g := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: fmt.Sprintf("kors_%s_%s", service, name),
		Help: help,
	})
	reg.MustRegister(g)
	return g
}
