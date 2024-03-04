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
	"io"
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
	ctx         context.Context
	mutex       sync.RWMutex
	pool        *pool.Pool
	host        string
	credProfile string

	up            prometheus.Gauge
	deviceMetrics *map[string]*metrics
}

// NewExporter returns an initialized Exporter for HPE DL380 device.
func NewExporter(ctx context.Context, target, uri, profile string) *Exporter {
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
			Scheme: config.GetConfig().BMCScheme,
			Host:   target,
		}
	}

	// vars for drive parsing
	var (
		initialURL        = "/Systems/1/SmartStorage/ArrayControllers/"
		url               = initialURL
		chassisUrl        = "/Chassis/1/"
		logicalDriveURLs  []string
		physicalDriveURLs []string
		nvmeDriveURLs     []string
	)

	// PARSING DRIVE ENDPOINTS
	// Get initial JSON return of /redfish/v1/Systems/1/SmartStorage/ArrayControllers/ set to output
	output, err := getDriveEndpoint(fqdn.String()+uri+url, target, retryClient)

	// Loop through Members to get ArrayController URLs
	if err != nil {
		log.Error("api call "+fqdn.String()+uri+url+" failed - ", zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		return nil
	}

	if output.MembersCount > 0 {
		for _, member := range output.Members {
			// for each ArrayController URL, get the JSON object
			newOutput, err := getDriveEndpoint(fqdn.String()+member.URL, target, retryClient)
			if err != nil {
				log.Error("api call "+fqdn.String()+member.URL+" failed - ", zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
				continue
			}

			// If LogicalDrives is present, parse logical drive endpoint until all urls are found
			if newOutput.Links.LogicalDrives.URL != "" {
				logicalDriveOutput, err := getDriveEndpoint(fqdn.String()+newOutput.Links.LogicalDrives.URL, target, retryClient)
				if err != nil {
					log.Error("api call "+fqdn.String()+newOutput.Links.LogicalDrives.URL+" failed - ", zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
					continue
				}

				if logicalDriveOutput.MembersCount > 0 {
					// loop through each Member in the "LogicalDrive" field
					for _, member := range logicalDriveOutput.Members {
						// append each URL in the Members array to the logicalDriveURLs array.
						logicalDriveURLs = append(logicalDriveURLs, member.URL)
					}
				}
			}

			// If PhysicalDrives is present, parse physical drive endpoint until all urls are found
			if newOutput.Links.PhysicalDrives.URL != "" {
				physicalDriveOutput, err := getDriveEndpoint(fqdn.String()+newOutput.Links.PhysicalDrives.URL, target, retryClient)

				if err != nil {
					log.Error("api call "+fqdn.String()+newOutput.Links.PhysicalDrives.URL+" failed - ", zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
					continue
				}
				if physicalDriveOutput.MembersCount > 0 {
					for _, member := range physicalDriveOutput.Members {
						physicalDriveURLs = append(physicalDriveURLs, member.URL)
					}
				}
			}
		}
	}

	// parse to find NVME drives
	chassisOutput, err := getDriveEndpoint(fqdn.String()+uri+chassisUrl, target, retryClient)
	if err != nil {
		log.Error("api call "+fqdn.String()+uri+chassisUrl+" failed - ", zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		return nil
	}

	// parse through "Links" to find "Drives" array
	if len(chassisOutput.Links.Drives) > 0 {
		// loop through drives array and append each odata.id url to nvmeDriveURLs list
		for _, drive := range chassisOutput.Links.Drives {
			nvmeDriveURLs = append(nvmeDriveURLs, drive.URL)
		}
	}

	// Loop through logicalDriveURLs, physicalDriveURLs, and nvmeDriveURLs and append each URL to the tasks pool
	for _, url := range logicalDriveURLs {
		tasks = append(tasks, pool.NewTask(common.Fetch(fqdn.String()+url, LOGICALDRIVE, target, profile, retryClient)))
	}

	for _, url := range physicalDriveURLs {
		tasks = append(tasks, pool.NewTask(common.Fetch(fqdn.String()+url, DISKDRIVE, target, profile, retryClient)))
	}

	for _, url := range nvmeDriveURLs {
		tasks = append(tasks, pool.NewTask(common.Fetch(fqdn.String()+url, NVME, target, profile, retryClient)))
	}

	// Additional tasks for pool to perform
	tasks = append(tasks,
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Chassis/1/Thermal/", THERMAL, target, profile, retryClient)),
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Chassis/1/Power/", POWER, target, profile, retryClient)),
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Systems/1/", MEMORY, target, profile, retryClient)))

	// Prepare the pool of tasks
	p := pool.NewPool(tasks, 1)

	// Create new map[string]*metrics for each new Exporter
	metrx := NewDeviceMetrics()

	return &Exporter{
		ctx:         ctx,
		pool:        p,
		host:        fqdn.Host,
		credProfile: profile,
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
					Name:              e.host,
					Endpoint:          "https://" + e.host + "/redfish/v1/Chassis",
					Module:            DL380,
					CredentialProfile: e.credProfile,
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

	if dlphysical.Status.Health == "OK" {
		state = OK
	} else {
		state = BAD
	}

	// Physical drives need to have a unique identifier like location so as to not overwrite data
	// physical drives can have the same ID, but belong to a different ArrayController, therefore need more than just the ID as a unique identifier.
	(*dlphysicaldrive)["driveStatus"].WithLabelValues(dlphysical.Name, dlphysical.Id, dlphysical.Location).Set(state)
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
	if dllogical.Status.Health == "OK" {
		state = OK
	} else {
		state = BAD
	}

	(*dllogicaldrive)["raidStatus"].WithLabelValues(dllogical.Name, dllogical.LogicalDriveName, dllogical.VolumeUniqueIdentifier, dllogical.Raid).Set(state)
	return nil
}

// exportNVMeDriveMetrics collects the DL380 NVME drive metrics in json format and sets the prometheus gauges
func (e *Exporter) exportNVMeDriveMetrics(body []byte) error {
	var state float64
	var dlnvme NVMeDriveMetrics
	var dlnvmedrive = (*e.deviceMetrics)["nvmeMetrics"]
	err := json.Unmarshal(body, &dlnvme)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling DL380 NVMeDriveMetrics - " + err.Error())
	}

	// Check nvme drive is enabled then check status and convert string to numeric values
	if dlnvme.Status.Health == "OK" {
		state = OK
	} else {
		state = BAD
	}

	(*dlnvmedrive)["nvmeDriveStatus"].WithLabelValues(dlnvme.Protocol, dlnvme.ID, dlnvme.PhysicalLocation.PartLocation.ServiceLabel).Set(state)
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

// The getDriveEndpoint function is used in a recursive fashion to get the body response
// of any type of drive, NVMe, Physical DiskDrives, or Logical Drives, using the GenericDrive struct
// This is used to find the final drive endpoints to append to the task pool for final scraping.
func getDriveEndpoint(url, host string, client *retryablehttp.Client) (GenericDrive, error) {
	var drive GenericDrive
	var resp *http.Response
	var err error
	retryCount := 0
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return drive, fmt.Errorf("Error reading Response Body - " + err.Error())
	}

	err = json.Unmarshal(body, &drive)
	if err != nil {
		return drive, fmt.Errorf("Error Unmarshalling DL380 drive struct - " + err.Error())
	}

	return drive, nil
}
