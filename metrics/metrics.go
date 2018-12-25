package metrics

import (
	"context"
	"flag"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.jonnrb.io/egress/log"
)

var (
	metricScrapeInterval = flag.Duration(
		"metrics.scrape_interval",
		5*time.Second,
		"How often to scrape metrics from the kernel.")
)

type Config struct {
	UplinkName string
}

type metrics struct {
	receiveBytes  prometheus.Gauge
	transmitBytes prometheus.Gauge
}

// Returns a metrics handler that will scrape metrics during ctx.
func New(ctx context.Context, cfg Config) (http.Handler, error) {
	m := metrics{
		receiveBytes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "uplink_network_receive_bytes",
			Help: "Counter reporting receive bytes on the uplink interface.",
		}),
		transmitBytes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "uplink_network_transmit_bytes",
			Help: "Counter reporting transmit bytes on the uplink interface.",
		}),
	}

	r := prometheus.NewRegistry()
	if err := r.Register(m.receiveBytes); err != nil {
		return nil, err
	}
	if err := r.Register(m.transmitBytes); err != nil {
		return nil, err
	}

	go scrapeOnInterval(ctx, m, cfg.UplinkName)

	return promhttp.HandlerFor(r, promhttp.HandlerOpts{}), nil
}

func scrapeOnInterval(ctx context.Context, m metrics, uplinkName string) {
	log.V(2).Infof("scraping metrics every %v", *metricScrapeInterval)

	t := time.NewTicker(*metricScrapeInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			doMetricsScrape(m, uplinkName)
		}
	}
}
