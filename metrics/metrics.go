package metrics

import (
	"context"
	"flag"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.jonnrb.io/egress/fw"
	"go.jonnrb.io/egress/log"
)

var (
	metricReceiveBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "uplink_network_receive_bytes",
		Help: "Counter reporting receive bytes on the uplink interface.",
	})
	metricTransmitBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "uplink_network_transmit_bytes",
		Help: "Counter reporting transmit bytes on the uplink interface.",
	})
)

var (
	metricScrapeInterval = flag.Duration(
		"metrics.scrape_interval",
		5*time.Second,
		"How often to scrape metrics from the kernel.")
)

// Returns a metrics handler that will scrape metrics during ctx.
func New(ctx context.Context, cfg fw.Config) (http.Handler, error) {
	if err := prometheus.Register(metricReceiveBytes); err != nil {
		return nil, err
	}
	if err := prometheus.Register(metricTransmitBytes); err != nil {
		return nil, err
	}

	go scrapeOnInterval(ctx, cfg.Uplink().Name())

	return promhttp.Handler(), nil
}

func scrapeOnInterval(ctx context.Context, uplinkName string) {
	log.V(2).Infof("scraping metrics every %v", *metricScrapeInterval)

	t := time.NewTicker(*metricScrapeInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			doMetricsScrape(uplinkName)
		}
	}
}
