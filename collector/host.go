package collector

import (
	"fmt"
	"log"
	"strings"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/lovoo/xenstat_exporter/xenapi"
	xsclient "github.com/xenserver/go-xenserver-client"
)

const (
	subsystem = "host"
)

func init() {
	Factories[subsystem] = NewHostCollector
}

// A HostCollector is a Prometheus collector for XenServer Host metrics
type HostCollector struct {
	config    Config
	CPUsTotal *prometheus.Desc
	CPUsUsed  *prometheus.Desc
	CPUsFree  *prometheus.Desc
}

func NewHostCollector(config Config) (Collector, error) {
	return &HostCollector{
		config: config,
		CPUsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystem, "cpus_total"),
			"Total number of CPU cores on the Xenhost",
			[]string{"hostname"},
			nil,
		),
		CPUsUsed: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystem, "cpus_used"),
			"Used CPU cores on the Xenhost",
			[]string{"hostname"},
			nil,
		),
		CPUsFree: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystem, "cpus_free"),
			"Free CPU cores on the Xenhost",
			[]string{"hostname"},
			nil,
		),
	}, nil
}

// Collect sends the metric values for each metric
// to the provided prometheus Metric channel.
func (c *HostCollector) Collect(ch chan<- prometheus.Metric) error {
	if desc, err := c.collect(ch); err != nil {
		log.Println("[ERROR] failed collecting %v metrics:", subsystem, desc, err)
		return err
	}
	return nil
}

func (c *HostCollector) collect(ch chan<- prometheus.Metric) (*prometheus.Desc, error) {
	for _, host := range c.config {
		xapi := GetXenAPI(host.Xenhost)
		var data = &HostCPUMetrics{}

		if err := xapi.APICaller.queryHostCPU(data); err != nil {
			return nil, err
		}

		ch <- prometheus.MustNewConstMetric(
			c.CPUsTotal,
			prometheus.GaugeValue,
			float64(data.CPUsTotal),
			data.hostname,
		)
		ch <- prometheus.MustNewConstMetric(
			c.CPUsUsed,
			prometheus.GaugeValue,
			float64(data.CPUsUsed),
			data.hostname,
		)
		ch <- prometheus.MustNewConstMetric(
			c.CPUsFree,
			prometheus.GaugeValue,
			float64(data.CPUsFree),
			data.hostname,
		)
	}

	return nil, nil
}
