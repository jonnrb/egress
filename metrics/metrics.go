package metrics

import (
	"context"
	"flag"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.jonnrb.io/egress/ha"
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
	HAHandler  func(m ha.Member)
}

type metrics struct {
	receiveBytes  prometheus.Gauge
	transmitBytes prometheus.Gauge
	isLeader      prometheus.Gauge
	isFollower    prometheus.Gauge
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
		isLeader: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "is_leader",
			Help: "Reports if the current node is the leader in an HA deployment.",
		}),
		isFollower: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "is_follower",
			Help: "Reports if the current node is a follower in an HA deployment.",
		}),
	}

	r := prometheus.NewRegistry()
	if err := r.Register(m.receiveBytes); err != nil {
		return nil, err
	}
	if err := r.Register(m.transmitBytes); err != nil {
		return nil, err
	}
	if err := r.Register(m.isLeader); err != nil {
		return nil, err
	}
	if err := r.Register(m.isFollower); err != nil {
		return nil, err
	}

	if cfg.HAHandler != nil {
		cfg.HAHandler(haObserver(m))
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

type haObserver metrics

func (m haObserver) Lead(ctx context.Context, _ func(time.Duration) error) error {
	log.V(2).Infof("setting metric noting that we became the leader")
	m.isLeader.Inc()

	<-ctx.Done()

	log.V(2).Infof("setting metric noting that we are stepping down as leader")
	m.isLeader.Dec()

	return ctx.Err()
}

func (m haObserver) Follow(ctx context.Context, leader string) error {
	log.V(2).Infof("setting metric noting that %q became the leader", leader)
	m.isFollower.Inc()

	<-ctx.Done()

	log.V(2).Infof("setting metric noting that %q is no longer the leader", leader)
	m.isFollower.Dec()

	return ctx.Err()
}
