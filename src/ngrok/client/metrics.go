package client

import "ngrok/metrics"

type ClientMetrics struct {
	// metrics
	connGauge       metrics.Gauge
	connMeter       metrics.Meter
	connTimer       metrics.Timer
	proxySetupTimer metrics.Timer
	bytesIn         metrics.Histogram
	bytesOut        metrics.Histogram
	bytesInCount    metrics.Counter
	bytesOutCount   metrics.Counter
}

func NewClientMetrics() *ClientMetrics {
	return &ClientMetrics{
		connGauge:       metrics.NewGauge(),
		connMeter:       metrics.NewMeter(),
		connTimer:       metrics.NewTimer(),
		proxySetupTimer: metrics.NewTimer(),
		bytesIn:         metrics.NewHistogram(),
		bytesOut:        metrics.NewHistogram(),
		bytesInCount:    metrics.NewCounter(),
		bytesOutCount:   metrics.NewCounter(),
	}
}
