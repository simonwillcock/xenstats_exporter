package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/simonwillcock/xenstats_exporter/collector"
	"gopkg.in/yaml.v2"
)

// XenstatsCollector implements the prometheus.Collector interface.
type XenstatsCollector struct {
	collectors map[string]collector.Collector
}

const (
	defaultCollectors = "host" //,pool,cpu,mem,storage"
	defaultCollectorsPlaceholder = "[defaults]"
)

var (
	scrapeDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(collector.Namespace, "exporter", "collector_duration_seconds"),
		"xenstats: Duration of a collection.",
		[]string{"collector"},
		nil,
	)
	scrapeSuccessDesc = prometheus.NewDesc(
		prometheus.BuildFQName(collector.Namespace, "exporter", "collector_success"),
		"xenstats: Whether the collector was successful.",
		[]string{"collector"},
		nil,
	)
)

var (
	listenAddress = flag.String("web.listen", ":9290", "Address on which to expose metrics and web interface.")
	metricsPath   = flag.String("web.path", "/metrics", "Path under which to expose metrics.")
	configFile    = flag.String("config.file", "config.yml", "Config file Path")
	namespace     = flag.String("namespace", "xenstats", "Namespace for the xenexporter metrics.")
)

// Describe sends all the descriptors of the collectors included to
// the provided channel.
func (coll XenstatsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- scrapeDurationDesc
	ch <- scrapeSuccessDesc
}

// Collect sends the collected metrics from each of the collectors to
// prometheus. Collect could be called several times concurrently
// and thus its run is protected by a single mutex.
func (coll XenstatsCollector) Collect(ch chan<- prometheus.Metric) {
	wg := sync.WaitGroup{}
	wg.Add(len(coll.collectors))
	for name, c := range coll.collectors {
		go func(name string, c collector.Collector) {
			execute(name, c, ch)
			wg.Done()
		}(name, c)
	}
	wg.Wait()
}

func filterAvailableCollectors(collectors string) string {
	var availableCollectors []string
	for _, c := range strings.Split(collectors, ",") {
		_, ok := collector.Factories[c]
		if ok {
			availableCollectors = append(availableCollectors, c)
		}
	}
	return strings.Join(availableCollectors, ",")
}

func execute(name string, c collector.Collector, ch chan<- prometheus.Metric) {
	begin := time.Now()
	err := c.Collect(ch)
	duration := time.Since(begin)
	var success float64

	if err != nil {
		log.Errorf("ERROR: %s collector failed after %fs: %s", name, duration.Seconds(), err)
		success = 0
	} else {
		log.Debugf("OK: %s collector succeeded after %fs.", name, duration.Seconds())
		success = 1
	}
	ch <- prometheus.MustNewConstMetric(
		scrapeDurationDesc,
		prometheus.GaugeValue,
		duration.Seconds(),
		name,
	)
	ch <- prometheus.MustNewConstMetric(
		scrapeSuccessDesc,
		prometheus.GaugeValue,
		success,
		name,
	)
}

func expandEnabledCollectors(enabled string) []string {
	expanded := strings.Replace(enabled, defaultCollectorsPlaceholder, defaultCollectors, -1)
	separated := strings.Split(expanded, ",")
	unique := map[string]bool{}
	for _, s := range separated {
		if s != "" {
			unique[s] = true
		}
	}
	result := make([]string, 0, len(unique))
	for s, _ := range unique {
		result = append(result, s)
	}
	return result
}

func loadCollectors(list string) (map[string]collector.Collector, error) {
	collectors := map[string]collector.Collector{}
	enabled := expandEnabledCollectors(list)

	for _, name := range enabled {
		fn, ok := collector.Factories[name]
		if !ok {
			return nil, fmt.Errorf("collector '%s' not available", name)
		}
		c, err := fn()
		if err != nil {
			return nil, err
		}
		collectors[name] = c
	}
	return collectors, nil
}

func readConfig() (config Config, err error) {
	config = Config{}

	source, err := ioutil.ReadFile(*configFile)
	if err != nil {
		return config, fmt.Errorf("could not read config: %v", err)
	}

	err = yaml.Unmarshal(source, &config)
	if err != nil {
		return config, fmt.Errorf("could not unmarshal config: %v", err)
	}
	return config, err
}

func main() {
	flag.Parse()

	collectors, err := loadCollectors(*enabledCollectors)
	if err != nil {
		log.Fatalf("Couldn't load collectors: %s", err)
	}

	log.Infof("Enabled collectors: %v", strings.Join(keys(collectors), ", "))

	config, err := readConfig()
	if err != nil {
		log.Printf("%v", err)
		return
	}

	xenstatsCollector := XenstatsCollector{collectors: collectors}
	prometheus.MustRegister(xenstatsCollector)

	log.Printf("Starting Server: %s", *listenAddress)
	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/health", healthCheck)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, *metricsPath, http.StatusMovedPermanently)
	})

	go func() {
		log.Infoln("Starting server on", *listenAddress)
		log.Fatalf("cannot start Xenstats exporter: %s", http.ListenAndServe(*listenAddress, nil))
	}()
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, `{"status":"ok"}`)
}

func keys(m map[string]collector.Collector) []string {
	ret := make([]string, 0, len(m))
	for key := range m {
		ret = append(ret, key)
	}
	return ret
}
