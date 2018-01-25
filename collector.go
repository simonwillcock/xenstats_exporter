package main

import (
	"log"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// Exporter implements the prometheus.Collector interface. It exposes the metrics
// of a ipmi node.
type Exporter struct {
	exporters    []*ExporterHost
}

type ExporterHost struct {
	config       *HostConfig
	metrics      []*prometheus.GaugeVec
	totalScrapes prometheus.Counter
	replacer     *strings.Replacer
}

// HostConfig -
type Config struct {
	configs []*HostConfig
}

// HostConfig -
type HostConfig struct {
	Xenhost string

	Credentials struct {
		Username string
		Password string
	}
}

// NewExporter instantiates a new ipmi Exporter.
func NewExporter(config Config) *Exporter {

	exp := &Exporter{
		exporters: make([]*ExporterHost, 0, len(config.configs)),
	}

	for _, host := range config.configs {
		e := &ExporterHost{
			config: host,
		}
		e.metrics = []*prometheus.GaugeVec{}
		e.collect()
		exp.exporters = append(exp.exporters, e)
	}

	return exp
}

// Describe Describes all the registered stats metrics from the xen master.
func (exp *Exporter) Describe(ch chan<- *prometheus.Desc) {
	for _, e := range exp.exporters {
		if e == nil {
			continue
		}
		for _, metric := range e.metrics {
			metric.Describe(ch)
		}
	}
}

// Collect collects all the registered stats metrics from the xen master.
func (exp *Exporter) Collect(metrics chan<- prometheus.Metric) {
	for _, e := range exp.exporters {
		if e == nil {
			continue
		}
		e.collect()
		for _, m := range e.metrics {
			m.Collect(metrics)
		}
	}
}

func (e *ExporterHost) collect() {
	var err error

	stats := NewXenstats(e.config)
	stats.GetApiCaller()

	e.metrics, err = stats.createHostMemMetrics()
	if err != nil {
		log.Printf("Xen api error in creating host memory metrics: %v", err)
	}

	poolmetrics, err := stats.createPoolMetrics()
	if err != nil {
		log.Printf("Xen api error in creating ha pool metrics: %v", err)
	}
	e.metrics = append(e.metrics, poolmetrics...)

	storagemetrics, err := stats.createStorageMetrics()
	if err != nil {
		log.Printf("Xen api error in creating storage metrics: %v", err)
	}
	e.metrics = append(e.metrics, storagemetrics...)

	cpumetrics, err := stats.createHostCPUMetrics()
	if err != nil {
		log.Printf("Xen api error in creating host cpu metrics: %v", err)
	}
	e.metrics = append(e.metrics, cpumetrics...)

	err = stats.CloseApi()
	if err != nil {
		log.Printf("Error during connection close: %v", err)
	}
}
