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
	//// DRIVE represents the logical drive metric endpoints
	//DRIVE = "DriveMetrics"
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
	// TODO: iterate through ArrayControllers:
	// List of everything passed to each common.Fetch: e.g.: (fqdn.String()+uri+"/Chassis/1/Thermal", THERMAL, target, retryClient)
	// For each item in list, parse with new common.FetchIterate, returning a list of every endpoint needed
	// then iterate through the list, creating pool.NewTask for each, putting in list newTasks
	// then end with (?) tasks = append(tasks, newTasks)
	// all logic of finding "links" and parsing whether LogicalDrives or DiskDrives has members will be handled here

	tasks = append(tasks,
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Chassis/1/Thermal", THERMAL, target, retryClient)),
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Chassis/1/Power", POWER, target, retryClient)),
		pool.NewTask(common.Fetch(fqdn.String()+uri+"Chassis/1", NVME, target, retryClient)),
		// if a logical drive, it will be in ArrayControllers/x/ -> Links -> DiskDrives/x/
		// example: /redfish/v1/Systems/1/SmartStorage/ArrayControllers/2/DiskDrives/0/
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Systems/1/SmartStorage/ArrayControllers", DISKDRIVE, target, retryClient)),
		// if a logical drive, it will be in ArrayControllers/x/ -> Links -> LogicalDrives/x/
		// example: /redfish/v1/Systems/1/SmartStorage/ArrayControllers/3/LogicalDrives/1/
		// pool.NewTask(common.Fetch(fqdn.String()+uri+"/Systems/1/SmartStorage/ArrayControllers/0/LogicalDrives/1", DRIVE, target, retryClient)),
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Systems/1/SmartStorage/ArrayControllers", LOGICALDRIVE, target, retryClient)),
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Systems/1", MEMORY, target, retryClient)))

	// tasks need to be refactored, so that tasks will need to be iterated over
	// TODO:

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
		// TODO: does the DRIVE case need to be split into NVME, DDRIVE, LDRIVE?
		// case DRIVE:
		// 	err = e.exportDriveMetrics(task.Body)
		case NVME:
			err = e.exportNVMeDriveMetrics(task.Body)
		case DISKDRIVE:
			err = e.exportDiskDriveMetrics(task.Body)
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

// TODO: Modify the below PowerMetrics to fit the DL380 data
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

// exportNVMeDriveMetrics collects the DL380 NVME drive metrics in json format and sets the prometheus gauges
func (e *Exporter) exportNVMeDriveMetrics(body []byte) error {

	var state float64
	var dlnvme NVMeDriveMetrics
	var dlnvmedrive = (*e.deviceMetrics)["nvmeDriveMetrics"]
	err := json.Unmarshal(body, &dlnvme)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling DL380 NVMeDriveMetrics - " + err.Error())
	}
	// Check logical drive is enabled then check status and convert string to numeric values
	if dlnvme.Status.State == "Enabled" {
		if dlnvme.Status.Health == "OK" {
			state = OK
		} else {
			state = BAD
		}
	} else {
		state = DISABLED
	}

	(*dlnvmedrive)["nvmeDriveStatus"].WithLabelValues(dlnvme.Protocol, dlnvme.ID, dlnvme.Name).Set(state)

	return nil
}

// exportDriveMetrics collects the DL380 drive metrics in json format and sets the prometheus gauges
func (e *Exporter) exportDiskDriveMetrics(body []byte) error {

	var state float64
	var dd DiskDriveMetrics
	var dDrive = (*e.deviceMetrics)["diskDriveMetrics"]
	err := json.Unmarshal(body, &dd)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling DL380 DiskDriveMetrics - " + err.Error())
	}

	// Check disk drive is enabled then check status and convert string to numeric values
	if dd.Status.State == "Enabled" {
		if dd.Status.Health == "OK" {
			state = OK
		} else {
			state = BAD
		}
	} else {
		state = DISABLED
	}

	// Check "disk drive" if it is actually a logical drive
	if dd.LogicalDriveName == "" {
		// if drive is actually a logical drive, then skip it, otherwise, add metrics.
		(*dDrive)["diskDriveStatus"].WithLabelValues(dd.Name, strconv.Itoa(dd.CapacityMiB)).Set(state)
	}

	return nil
}

// exportDriveMetrics collects the DL380 logicaldrive metrics in json format and sets the prometheus gauges
func (e *Exporter) exportLogicalDriveMetrics(body []byte) error {

	var state float64
	var dld LogicalDriveMetrics
	var dlDrive = (*e.deviceMetrics)["logicalDriveMetrics"]
	err := json.Unmarshal(body, &dld)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling DL380 DriveMetrics - " + err.Error())
	}
	// Check logical drive is enabled then check status and convert string to numeric values
	if dld.Status.State == "Enabled" {
		if dld.Status.Health == "OK" {
			state = OK
		} else {
			state = BAD
		}
	} else {
		state = DISABLED
	}
	// Check if drive is actually a logical drive
	if dld.LogicalDriveName != "" {
		// if drive is actually a logical drive, then add metrics.
		(*dlDrive)["logicalDriveStatus"].WithLabelValues(dld.Name, strconv.Itoa(dld.LogicalDriveNumber), dld.Raid).Set(state)
	}

	return nil
}

// TODO: Modify the below MemoryMetrics to fit the DL380 data
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
