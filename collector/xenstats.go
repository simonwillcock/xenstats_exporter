ackage collector

import (
	"bytes"
	"reflect"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// ...
const (
	Namespace = "xenstats"

	// Conversion factors
	ticksToSecondsScaleFactor = 1 / 1e7
)

// Factories ...
var (
	Factories = make(map[string]func() (Collector, error))
	XenClients = make(map[string]) XenAPI)
)

// Collector is the interface a collector has to implement.
type Collector interface {
	// Get new metrics and expose them via prometheus registry.
	Collect(ch chan<- prometheus.Metric) (err error)
}

type Config struct {
	Xenhosts []*HostConfig
}

type HostConfig struct {
	Xenhost string

	Credentials struct {
		Username string
		Password string
	}
}

// APIConnection hold information about the Xenserver and the credentials
type APIConnection struct {
	Server       string
	Username     string
	Password     string
	xenAPIClient *xsclient.XenAPIClient
}

type XenAPI struct {
	APICaller      *APIConnection
	APIClient *xsclient.XenAPIClient
}

// ApiObject of type ..
type ApiObject xsclient.XenAPIObject

// NewAPIConnection Creates a new APIConnection
func CreateAPIConnection(host, username, password string) *APIConnection {
	return &APIConnection{
		Server:   host,
		Username: username,
		Password: password,
	}
}

func GetXenAPI(config *HostConfig) *XenAPI {
	client, ok := XenClients[config.Xenhost]
	if !ok {
		client = CreateXenApI(config)
		XenClients[config.Xenhost] = client
	}
	return client;
}

func CreateXenApI(config *HostConfig) *XenAPI {
	conn := new(APIConnection)

	caller := CreateAPIConnection(config.Xenhost, config.Credentials.Username, config.Credentials.Password)

	// Need Login first if it is a fresh session
	client, err := caller.GetXenAPIClient()
	if err != nil {
		log.Printf("service.time call error: %v", err)
	}

	conn.APIClient = client
	conn.APICaller = caller

	return conn
}

// GetXenAPIClient returns
func (d *APIConnection) GetXenAPIClient() (*xsclient.XenAPIClient, error) {
	var err error
	if d.xenAPIClient == nil {
		c, err := d.newXenAPIClient()
		if err != nil {
			return nil, err
		}
		if err := c.Login(); err != nil {
			return nil, err
		}
		d.xenAPIClient = &c
	}
	return d.xenAPIClient, err
}


// GetSpecificValue -
func (d *APIConnection) GetSpecificValue(apikey string, params string) (interface{}, error) {
	result := xsclient.APIResult{}
	err := d.xenAPIClient.APICall(&result, apikey, params)

	return result.Value, err
}

// GetMultiValues -
func (d *APIConnection) GetMultiValues(apikey string, params ...string) (apiObjects []*ApiObject, err error) {
	result := xsclient.APIResult{}

	if len(params) > 0 {
		err = d.xenAPIClient.APICall(&result, apikey, params[0])
	} else {
		err = d.xenAPIClient.APICall(&result, apikey)
	}

	if err != nil {
		return apiObjects, err
	}

	for _, elem := range result.Value.([]interface{}) {
		apiObject := new(ApiObject)
		apiObject.Ref = elem.(string)
		apiObjects = append(apiObjects, apiObject)
	}

	return apiObjects, err
}

type HostCPUMetrics struct {
	hostname    string
	CPUsTotal   uint64
	CPUsUsed    uint64
	CPUsFree    uint64
}

func (client APIClient) queryHostCPU() (data *HostCPUMetrics, err error) {
	hosts, err := client.GetHosts()
	if err != nil {
		// log.Printf("service.time call error: %v", err)
		return data, fmt.Errorf("XEN Api Error: %v", err)
	}
	for _, elem := range hosts {
		usedCpus := int64(0)
		vmsPerHost := float64(0)
		hostname, err := s.xend.GetSpecificValue("host.get_name_label", elem.Ref)
		if err != nil {
			return data, fmt.Errorf("XEN Api Error: %v", err)
		}
		hostcpus, err := s.xend.GetMultiValues("host.get_host_CPUs", elem.Ref)
		if err != nil {
			return data, fmt.Errorf("XEN Api Error: %v", err)
		}
		vms, err := s.xend.GetMultiValues("host.get_resident_VMs", elem.Ref)
		if err != nil {
			return data, fmt.Errorf("XEN Api Error: %v", err)
		}
		for _, elem2 := range vms {

			vmIsControllDomain, err := s.xend.GetSpecificValue("VM.get_is_control_domain", elem2.Ref)
			if err != nil {
				return data, fmt.Errorf("XEN Api Error: %v", err)
			}

			if vmIsControllDomain.(bool) == false {
				vmmetrics, err := s.xend.GetSpecificValue("VM.get_metrics", elem2.Ref)
				if err != nil {
					return data, fmt.Errorf("XEN Api Error: %v", err)
				}

				vmCPUCount, err := s.xend.GetSpecificValue("VM_metrics.get_VCPUs_number", vmmetrics.(string))
				if err != nil {
					return data, fmt.Errorf("XEN Api Error: %v", err)
				}
				vmCPUCountint, err := strconv.ParseInt(vmCPUCount.(string), 10, 64)
				if err != nil {
					return data, fmt.Errorf("value conversation error: %v", err)
				}

				usedCpus += vmCPUCountint
				vmsPerHost++
			}
		}

		cpusFree := int64(len(hostcpus)) - usedCpus
		cpuUtilPercent := 100 * usedCpus / int64(len(hostcpus))
		data.hostname = hostname
		data.CPUsTotal = int64(len(hostcpus))
		data.CPUsUsed = usedCpus
		data.CPUsFree = int64(len(hostcpus)) - usedCpus

	}
	return data, err
}
