/*
 * Copyright 2024 Comcast Cable Communications Management, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package dl380

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/comcast/fishymetrics/common"
	"github.com/comcast/fishymetrics/config"
	"github.com/comcast/fishymetrics/pool"
	"go.uber.org/zap"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// DL380 is a HPE Hardware Device we scrape
	DL380 = "DL380"
	// THERMAL represents the thermal metric endpoint
	THERMAL = "ThermalMetrics"
	// POWER represents the power metric endpoint
	POWER = "PowerMetrics"
	// NVME represents the NVMe drive metric endpoint
	NVME = "NVMeDriveMetrics"
	// DISKDRIVE represents the Disk Drive metric endpoints
	DISKDRIVE = "DiskDriveMetrics"
	// LOGICALDRIVE represents the Logical drive metric endpoint
	LOGICALDRIVE = "LogicalDriveMetrics"
	// MEMORY represents the memory metric endpoints
	MEMORY = "MemoryMetrics"
	// OK is a string representation of the float 1.0 for device status
	OK = 1.0
	// BAD is a string representation of the float 0.0 for device status
	BAD = 0.0
	// DISABLED is a string representation of the float -1.0 for device status
	DISABLED = -1.0
)

var (
	log *zap.Logger
)

// Exporter collects chassis manager stats from the given URI and exports them using
// the prometheus metrics package.
type Exporter struct {
	ctx   context.Context
	mutex sync.RWMutex
	pool  *pool.Pool
	host  string

	up            prometheus.Gauge
	deviceMetrics *map[string]*metrics
}

// NewExporter returns an initialized Exporter for HPE DL380 device.
func NewExporter(ctx context.Context, target, uri string) *Exporter {
	var fqdn *url.URL
	var tasks []*pool.Task

	log = zap.L()

	tr := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 3 * time.Second,
		}).Dial,
		MaxIdleConns:          1,
		MaxConnsPerHost:       1,
		MaxIdleConnsPerHost:   1,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		TLSHandshakeTimeout: 10 * time.Second,
	}

	retryClient := retryablehttp.NewClient()
	retryClient.CheckRetry = retryablehttp.ErrorPropagatedRetryPolicy
	retryClient.HTTPClient.Transport = tr
	retryClient.HTTPClient.Timeout = 30 * time.Second
	retryClient.Logger = nil
	retryClient.RetryWaitMin = 2 * time.Second
	retryClient.RetryWaitMax = 2 * time.Second
	retryClient.RetryMax = 2
	retryClient.RequestLogHook = func(l retryablehttp.Logger, r *http.Request, i int) {
		retryCount := i
		if retryCount > 0 {
			log.Error("api call "+r.URL.String()+" failed, retry #"+strconv.Itoa(retryCount), zap.Any("trace_id", ctx.Value("traceID")))
		}
	}

	// Check that the target passed in has http:// or https:// prefixed
	fqdn, err := url.ParseRequestURI(target)
	if err != nil {
		fqdn = &url.URL{
			Scheme: config.GetConfig().OOBScheme,
			Host:   target,
		}
	}

	// vars for drive parsing
	var (
		initialURL        = "/Systems/1/SmartStorage/ArrayControllers"
		url               = initialURL
		chassis_url       = "/Chassis/1"
		logicalDriveURLs  []string
		physicalDriveURLs []string
		nvmeDriveURLs     []string
	)

	// PARSING DRIVE ENDPOINTS
	// Get initial JSON return of /redfish/v1/Systems/1/SmartStorage/ArrayControllers/ set to output
	output, err := getDriveEndpoint(fqdn.String()+uri+url, target, retryClient)

	// DEBUG PRINT
	fmt.Print("OUTPUT: ", output, "\n")
	// DEBUG PRINT

	// Loop through Members to get ArrayController URLs
	if err != nil {
		// DEBUG PRINT
		fmt.Print("ERROR LOOPING THROUGH ARRAY CONTROLLERS URLS", "\n")
		// DEBUG PRINT
		return nil
	}

	// DEBUG PRINT
	fmt.Print("OUTPUT.MEMBERSCOUNT: ", output.MembersCount, "\n")
	fmt.Print("OUTPUT.MEMBERS: ", output.Members, "\n")
	// DEBUG PRINT

	if output.MembersCount > 0 {
		// DEBUG PRINT
		fmt.Print("output.MembersCount > 0 : ", output.MembersCount, "\n")
		// DEBUG PRINT
		for _, member := range output.Members {
			// for each ArrayController URL, get the JSON object
			newOutput, err := getDriveEndpoint(fqdn.String()+member.URL, target, retryClient)
			if err != nil {
				// TODO: error handle
				continue
			}

			// If LogicalDrives is present, parse logical drive endpoint until all urls are found
			if newOutput.Links.LogicalDrives.URL != "" {
				logicalDriveOutput, err := getDriveEndpoint(fqdn.String()+newOutput.Links.LogicalDrives.URL, target, retryClient)
				if err != nil {
					// TODO: error handle
					// DEBUG PRINT
					fmt.Print("newOutput.Links.LogicalDrives.URL: ", newOutput.Links.LogicalDrives.URL, "\n")
					// DEBUG PRINT
					continue
				}

				if logicalDriveOutput.MembersCount > 0 {
					// loop through each Member in the "LogicalDrive" field
					for _, member := range logicalDriveOutput.Members {
						// append each URL in the Members array to the logicalDriveURLs array.
						logicalDriveURLs = append(logicalDriveURLs, member.URL)
						// DEBUG PRINT
						fmt.Print("Member in logicalDriveOutput.Members: ", member, "\n")
						fmt.Print("Members physicalDriveOutput.Members: ", logicalDriveOutput.Members, "\n")
						fmt.Print("member.URL: ", member.URL, "\n")
						// DEBUG PRINT
					}
				}
			}

			// If PhysicalDrives is present, parse physical drive endpoint until all urls are found
			if newOutput.Links.PhysicalDrives.URL != "" {
				physicalDriveOutput, err := getDriveEndpoint(fqdn.String()+newOutput.Links.PhysicalDrives.URL, target, retryClient)
				// DEBUG PRINT
				fmt.Print("newOutput.Links.PhysicalDrives.URL: ", newOutput.Links.PhysicalDrives.URL, "\n")
				// DEBUG PRINT

				if err != nil {
					// TODO: error handle
					continue
				}
				if physicalDriveOutput.MembersCount > 0 {
					for _, member := range physicalDriveOutput.Members {
						physicalDriveURLs = append(physicalDriveURLs, member.URL)
						// DEBUG PRINT
						fmt.Print("Member in physicalDriveOutput.Members: ", member, "\n")
						fmt.Print("Members physicalDriveOutput.Members: ", physicalDriveOutput.Members, "\n")
						fmt.Print("member.URL: ", member.URL, "\n")
						// DEBUG PRINT
					}
				}
			}
		}
	}

	// parse to find NVME drives
	chassis_output, err := getDriveEndpoint(fqdn.String()+uri+chassis_url, target, retryClient)
	if err != nil {
		// TODO: error handle
		return nil
	}

	// parse through "Links" to find "Drives" array
	if len(chassis_output.Links.Drives) > 0 {
		// loop through drives array and append each odata.id url to nvmeDriveURLs list
		for _, drive := range chassis_output.Links.Drives {
			nvmeDriveURLs = append(nvmeDriveURLs, drive.URL)
		}
	}

	// Loop through logicalDriveURLs, physicalDriveURLs, and nvmeDriveURLs and append each URL to the tasks pool
	for _, url := range logicalDriveURLs {
		tasks = append(tasks, pool.NewTask(common.Fetch(fqdn.String()+url, LOGICALDRIVE, target, retryClient)))
		// DEBUG PRINT
		fmt.Print("LOGICAL DRIVE URL: ", url, "\n")
		//fmt.Print("FQDN STRING: ", fqdn.String(), "\n")
		// DEBUG PRINT
	}

	for _, url := range physicalDriveURLs {
		tasks = append(tasks, pool.NewTask(common.Fetch(fqdn.String()+url, DISKDRIVE, target, retryClient)))
		// DEBUG PRINT
		fmt.Print("PHYSICAL DRIVE URL: ", url, "\n")
		//fmt.Print("FQDN STRING: ", fqdn.String(), "\n")
		// DEBUG PRINT
	}

	for _, url := range nvmeDriveURLs {
		tasks = append(tasks, pool.NewTask(common.Fetch(fqdn.String()+url, NVME, target, retryClient)))
		// DEBUG PRINT
		fmt.Print("NVME DRIVE URL: ", url, "\n")
		//fmt.Print("FQDN STRING: ", fqdn.String(), "\n")
		// DEBUG PRINT
	}

	// DEBUG PRINT
	fmt.Print("NVME DRIVE URLS LIST: ", nvmeDriveURLs, "\n")
	fmt.Print("LOGICAL DRIVE URLS LIST: ", logicalDriveURLs, "\n")
	fmt.Print("PHYSICAL DRIVE URLS LIST: ", physicalDriveURLs, "\n")
	// DEBUG PRINT

	// Additional tasks for pool to perform
	tasks = append(tasks,
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Chassis/1/Thermal", THERMAL, target, retryClient)),
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Chassis/1/Power", POWER, target, retryClient)),
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Systems/1", MEMORY, target, retryClient)))

	// DEBUG PRINT
	for _, task := range tasks {
		fmt.Print("TASK: ", task, "\n")
	}
	// DEBUG PRINT

	// DEBUG PRINT
	fmt.Print("TASK LIST: ", tasks, "\n")
	fmt.Print("LENGTH OF TASK LIST: ", len(tasks), "\n")
	// DEBUG PRINT

	// Prepare the pool of tasks
	p := pool.NewPool(tasks, 1)

	// Create new map[string]*metrics for each new Exporter
	metrx := NewDeviceMetrics()

	return &Exporter{
		ctx:  ctx,
		pool: p,
		host: fqdn.Host,
		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "up",
			Help: "Was the last scrape of chassis monitor successful.",
		}),
		deviceMetrics: metrx,
	}
}

// Describe describes all the metrics ever exported by the fishymetrics exporter. It
// implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range *e.deviceMetrics {
		for _, n := range *m {
			n.Describe(ch)
		}
	}
	ch <- e.up.Desc()
}

// Collect fetches the stats from configured fishymetrics location and delivers them
// as Prometheus metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock() // To protect metrics from concurrent collects.
	defer e.mutex.Unlock()

	e.resetMetrics()

	// perform scrape if target is not on ignored list
	if _, ok := common.IgnoredDevices[e.host]; !ok {
		e.scrape()
	} else {
		e.up.Set(float64(2))
	}

	ch <- e.up
	e.collectMetrics(ch)
}

func (e *Exporter) resetMetrics() {
	for _, m := range *e.deviceMetrics {
		for _, n := range *m {
			n.Reset()
		}
	}
}

func (e *Exporter) collectMetrics(metrics chan<- prometheus.Metric) {
	for _, m := range *e.deviceMetrics {
		for _, n := range *m {
			n.Collect(metrics)
		}
	}
}

func (e *Exporter) scrape() {

	var result uint8
	state := uint8(1)
	scrapes := len(e.pool.Tasks)
	scrapeChan := make(chan uint8, scrapes)

	// Concurrently call the endpoints to help prevent reaching the maxiumum number of 4 simultaneous sessions
	e.pool.Run()
	for _, task := range e.pool.Tasks {
		var err error
		if task.Err != nil {
			deviceState := uint8(0)
			// If credentials are incorrect we will add host to be ignored until manual intervention
			if strings.Contains(task.Err.Error(), "401") {
				common.IgnoredDevices[e.host] = common.IgnoredDevice{
					Name:     e.host,
					Endpoint: "https://" + e.host + "/redfish/v1/Chassis",
					Module:   DL380,
				}
				log.Info("added host "+e.host+" to ignored list", zap.Any("trace_id", e.ctx.Value("traceID")))
				deviceState = 2
			} else {
				deviceState = 0
			}
			e.up.Set(float64(deviceState))
			log.Error("error from "+DL380, zap.Error(task.Err), zap.String("api", task.MetricType), zap.Any("trace_id", e.ctx.Value("traceID")))
			return
		}

		switch task.MetricType {
		case THERMAL:
			err = e.exportThermalMetrics(task.Body)
		case POWER:
			err = e.exportPowerMetrics(task.Body)
		case NVME:
			err = e.exportNVMeDriveMetrics(task.Body)
		case DISKDRIVE:
			err = e.exportPhysicalDriveMetrics(task.Body)
		case LOGICALDRIVE:
			err = e.exportLogicalDriveMetrics(task.Body)
		case MEMORY:
			err = e.exportMemoryMetrics(task.Body)
		}

		if err != nil {
			log.Error("error exporting metrics - from "+DL380, zap.Error(err), zap.String("api", task.MetricType), zap.Any("trace_id", e.ctx.Value("traceID")))
			continue
		}
		scrapeChan <- 1
	}

	// Get scrape results from goroutine(s) and perform bitwise AND, any failures should
	// result in a scrape failure
	for i := 0; i < scrapes; i++ {
		result = <-scrapeChan
		state &= result
	}

	e.up.Set(float64(state))

}

// exportPhysicalDriveMetrics collects the DL380's physical drive metrics in json format and sets the prometheus gauges
func (e *Exporter) exportPhysicalDriveMetrics(body []byte) error {

	var state float64
	var dlphysical DiskDriveMetrics
	var dlphysicaldrive = (*e.deviceMetrics)["diskDriveMetrics"]
	err := json.Unmarshal(body, &dlphysical)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling DL380 DiskDriveMetrics - " + err.Error())
	}
	// Check physical drive is enabled then check status and convert string to numeric values
	if dlphysical.Status.State == "Enabled" {
		if dlphysical.Status.Health == "OK" {
			state = OK
		} else {
			state = BAD
		}
	} else {
		state = DISABLED
	}
	// TODO: Fix export to prometheus to include driveStatus
	//(*dlphysicaldrive)["DiskDriveMetrics"].WithLabelValues(dlphysical.Name, dlphysical.Id).Set(state)
	(*dlphysicaldrive)["DiskDriveMetrics"].WithLabelValues(dlphysical.Status.Health).Set(state)
	return nil
}

// exportLogicalDriveMetrics collects the DL380's physical drive metrics in json format and sets the prometheus gauges
func (e *Exporter) exportLogicalDriveMetrics(body []byte) error {
	var state float64
	var dllogical LogicalDriveMetrics
	var dllogicaldrive = (*e.deviceMetrics)["logicalDriveMetrics"]
	err := json.Unmarshal(body, &dllogical)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling DL380 LogicalDriveMetrics - " + err.Error())
	}
	// Check physical drive is enabled then check status and convert string to numeric values

	// DEBUG PRINT
	fmt.Print("dllogicaldrive", dllogicaldrive, "\n")
	//fmt.Print("dllogicaldrive.Status.State", dllogical.Status.State, "\n")
	fmt.Print("dllogical.Raid", dllogical.Raid, "\n")
	fmt.Print("dllogical.Id", dllogical.Id, "\n")
	fmt.Print("dllogical.Name", dllogical.Name, "\n")
	fmt.Print("dllogical.Status", dllogical.Status, "\n")
	fmt.Print("dllogical.Status.Health", dllogical.Status.Health, "\n")
	//	fmt.Print("dllogical.Status.State", dllogical.Status.State, "\n")

	// DEBUG PRINT

	if dllogical.Status.Health == "OK" {
		state = OK
	} else {
		state = BAD
	}

	// TODO: Fix export to prometheus to include driveStatus and Raid
	//(*dllogicaldrive)["logicalSummary"].WithLabelValues(dllogical.Name, dllogical.Id, dllogical.Raid).Set(state)
	//(*dllogicaldrive)["LogicalDriveMetrics"].WithLabelValues(dllogical.Name, dllogical.Id, dllogical.Raid).Set(state)
	(*dllogicaldrive)["logicalSummary"].WithLabelValues(dllogical.Raid).Set(state)
	return nil
}

// exportNVMeDriveMetrics collects the DL380 NVME drive metrics in json format and sets the prometheus gauges
func (e *Exporter) exportNVMeDriveMetrics(body []byte) error {
	if e == nil || e.deviceMetrics == nil {
		return fmt.Errorf("Exporter or deviceMetrics is nil")
	}

	var state float64
	var dlnvme NVMeDriveMetrics

	var dlnvmedrive = (*e.deviceMetrics)["nvmeDriveMetrics"]
	err := json.Unmarshal(body, &dlnvme)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling DL380 NVMeDriveMetrics - " + err.Error())
	}
	// Check nvme drive is enabled then check status and convert string to numeric values
	if dlnvme.Status.State == "Enabled" {
		if dlnvme.Status.Health == "OK" {
			state = OK
		} else {
			state = BAD
		}
	} else {
		state = DISABLED
	}

	// TODO: Fix export to prometheus to include driveStatus
	(*dlnvmedrive)["nvmeDriveMetrics"].WithLabelValues(dlnvme.Protocol, dlnvme.ID, dlnvme.Name).Set(state)
	return nil
}

// exportPowerMetrics collects the DL380's power metrics in json format and sets the prometheus gauges
func (e *Exporter) exportPowerMetrics(body []byte) error {

	var state float64
	var pm PowerMetrics
	var dlPower = (*e.deviceMetrics)["powerMetrics"]
	err := json.Unmarshal(body, &pm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling DL380 PowerMetrics - " + err.Error())
	}

	for _, pc := range pm.PowerControl {
		(*dlPower)["supplyTotalConsumed"].WithLabelValues(pc.MemberID).Set(float64(pc.PowerConsumedWatts))
		(*dlPower)["supplyTotalCapacity"].WithLabelValues(pc.MemberID).Set(float64(pc.PowerCapacityWatts))
	}

	for _, ps := range pm.PowerSupplies {
		if ps.Status.State == "Enabled" {
			(*dlPower)["supplyOutput"].WithLabelValues(ps.MemberID, ps.SparePartNumber).Set(float64(ps.LastPowerOutputWatts))
			if ps.Status.Health == "OK" {
				state = OK
			} else {
				state = BAD
			}
			(*dlPower)["supplyStatus"].WithLabelValues(ps.MemberID, ps.SparePartNumber).Set(state)
		}
	}

	return nil
}

// exportThermalMetrics collects the DL380's thermal and fan metrics in json format and sets the prometheus gauges
func (e *Exporter) exportThermalMetrics(body []byte) error {

	var state float64
	var tm ThermalMetrics
	var dlThermal = (*e.deviceMetrics)["thermalMetrics"]
	err := json.Unmarshal(body, &tm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling DL380 ThermalMetrics - " + err.Error())
	}

	// Iterate through fans
	for _, fan := range tm.Fans {
		// Check fan status and convert string to numeric values
		if fan.Status.State == "Enabled" {
			(*dlThermal)["fanSpeed"].WithLabelValues(fan.Name).Set(float64(fan.Reading))
			if fan.Status.Health == "OK" {
				state = OK
			} else {
				state = BAD
			}
			(*dlThermal)["fanStatus"].WithLabelValues(fan.Name).Set(state)
		}
	}

	// Iterate through sensors
	for _, sensor := range tm.Temperatures {
		// Check sensor status and convert string to numeric values
		if sensor.Status.State == "Enabled" {
			(*dlThermal)["sensorTemperature"].WithLabelValues(strings.TrimRight(sensor.Name, " ")).Set(float64(sensor.ReadingCelsius))
			if sensor.Status.Health == "OK" {
				state = OK
			} else {
				state = BAD
			}
			(*dlThermal)["sensorStatus"].WithLabelValues(strings.TrimRight(sensor.Name, " ")).Set(state)
		}
	}

	return nil
}

// exportMemoryMetrics collects the DL380 drive metrics in json format and sets the prometheus gauges
func (e *Exporter) exportMemoryMetrics(body []byte) error {

	var state float64
	var dlm MemoryMetrics
	var dlMemory = (*e.deviceMetrics)["memoryMetrics"]
	err := json.Unmarshal(body, &dlm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling DL380 MemoryMetrics - " + err.Error())
	}
	// Check memory status and convert string to numeric values
	if dlm.MemorySummary.Status.HealthRollup == "OK" {
		state = OK
	} else {
		state = BAD
	}

	(*dlMemory)["memoryStatus"].WithLabelValues(strconv.Itoa(dlm.MemorySummary.TotalSystemMemoryGiB)).Set(state)

	return nil
}

func getDriveEndpoint(url, host string, client *retryablehttp.Client) (GenericDrive, error) {
	var drive GenericDrive
	var resp *http.Response
	var err error
	retryCount := 0
	// build the url
	//full_url := ("https://" + fqdn + uri + url)
	req := common.BuildRequest(url, host)
	resp, err = common.DoRequest(client, req)
	if err != nil {
		return drive, err
	}
	defer resp.Body.Close()
	if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
		if resp.StatusCode == http.StatusNotFound {
			for retryCount < 3 && resp.StatusCode == http.StatusNotFound {
				time.Sleep(client.RetryWaitMin)
				resp, err = common.DoRequest(client, req)
				retryCount = retryCount + 1
			}
			if err != nil {
				return drive, err
			} else if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
				return drive, fmt.Errorf("HTTP status %d", resp.StatusCode)
			}
		} else {
			return drive, fmt.Errorf("HTTP status %d", resp.StatusCode)
		}
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return drive, fmt.Errorf("Error reading Response Body - " + err.Error())
	}

	err = json.Unmarshal(body, &drive)
	if err != nil {
		return drive, fmt.Errorf("Error Unmarshalling DL380 drive struct - " + err.Error())
	}

	return drive, nil
}
