package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.jonnrb.io/egress/fw"
	"go.jonnrb.io/egress/log"
)

const (
	PROC_NET_DEV = "/proc/net/dev"
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
	metricScrapeInterval = flag.Duration(
		"metrics.scrape_interval",
		5*time.Second,
		"How often to scrape metrics from the kernel.")
)

type metricsScraper chan interface{}

func SetupMetrics(cfg fw.Config) (io.Closer, error) {
	if err := prometheus.Register(metricReceiveBytes); err != nil {
		return nil, err
	}

	if err := prometheus.Register(metricTransmitBytes); err != nil {
		return nil, err
	}

	http.Handle("/metrics", promhttp.Handler())

	scraper := metricsScraper(make(chan interface{}))
	scraper.scrapeOnInterval(cfg.Uplink().Name())

	return scraper, nil
}

func (m metricsScraper) Close() error {
	m <- nil
	close(m)
	return nil
}

func (m metricsScraper) scrapeOnInterval(uplinkName string) {
	log.V(2).Infof("scraping metrics every %v", *metricScrapeInterval)
	go func() {
		t := time.NewTimer(*metricScrapeInterval)
		for {
			select {
			case <-m:
				if !t.Stop() {
					<-t.C
				}
				return
			case <-t.C:
				doMetricsScrape(uplinkName)
				t.Reset(*metricScrapeInterval)
			}
		}
	}()
}

func doMetricsScrape(uplinkName string) {
	stats, err := getNetDevStats()
	if err != nil {
		log.Errorf("error scraping network stats: %v", err)
		return
	}

	ifaceStats, ok := stats[uplinkName]
	if !ok {
		log.Errorf("iface %q not found in kernel network stats table", uplinkName)
		return
	}

	receiveBytes, ok := ifaceStats["receive_bytes"]
	if !ok {
		log.Error("could not find receive_bytes stat")
		return
	}
	metricReceiveBytes.Set(float64(receiveBytes))

	transmitBytes, ok := ifaceStats["transmit_bytes"]
	if !ok {
		log.Error("could not find transmit_bytes stat")
		return
	}
	metricTransmitBytes.Set(float64(transmitBytes))
}

func getNetDevStats() (map[string]map[string]int64, error) {
	file, err := os.Open(PROC_NET_DEV)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// scan two lines (the weird looking headers)
	if !scanner.Scan() || !scanner.Scan() {
		return nil, fmt.Errorf("bad %v", PROC_NET_DEV)
	}

	headerParts := strings.Split(scanner.Text(), "|")
	if len(headerParts) != 3 {
		return nil, fmt.Errorf("bad header line in %v: %q", PROC_NET_DEV, scanner.Text())
	}
	rHeader, tHeader := strings.Fields(headerParts[1]), strings.Fields(headerParts[2])

	keys := make([]string, len(rHeader)+len(tHeader))
	for i, r := range rHeader {
		keys[i] = "receive_" + r
	}
	for i, t := range tHeader {
		keys[i+len(rHeader)] = "transmit_" + t
	}

	stats := make(map[string]map[string]int64)
	for scanner.Scan() {
		a := strings.Split(scanner.Text(), ":")
		if len(a) != 2 {
			return nil, fmt.Errorf("bad stats line: %q", scanner.Text())
		}
		iface, fields := strings.TrimSpace(a[0]), strings.Fields(a[1])
		ifaceStats := make(map[string]int64)
		for i, field := range fields {
			if n, err := strconv.ParseInt(field, 10, 64); err != nil {
				return nil, fmt.Errorf("error parsing number: %v", field)
			} else {
				ifaceStats[keys[i]] = n
			}
		}
		stats[iface] = ifaceStats
	}
	return stats, nil
}
