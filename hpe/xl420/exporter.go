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

package xl420

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
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
	// XL420 is a HPE Hardware Device we scrape
	XL420 = "XL420"
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
	// PROCESSOR represents the processor metric endpoints
	PROCESSOR = "ProcessorMetrics"
	// STORAGEBATTERY represents the processor metric endpoints
	STORAGEBATTERY = "storBatteryMetrics"
	// ILOSELFTEST represents the processor metric endpoints
	ILOSELFTEST = "iloSelfTestMetrics"
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
	ctx           context.Context
	mutex         sync.RWMutex
	pool          *pool.Pool
	host          string
	credProfile   string
	deviceMetrics *map[string]*metrics
}

// NewExporter returns an initialized Exporter for HPE XL420 device.
func NewExporter(ctx context.Context, target, uri, profile string) (*Exporter, error) {
	var fqdn *url.URL
	var tasks []*pool.Task
	var exp = Exporter{
		ctx:           ctx,
		credProfile:   profile,
		deviceMetrics: NewDeviceMetrics(),
	}

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
	exp.host = fqdn.String()

	// check if host is on the ignored list, if so we immediately return
	if _, ok := common.IgnoredDevices[exp.host]; ok {
		var upMetric = (*exp.deviceMetrics)["up"]
		(*upMetric)["up"].WithLabelValues().Set(float64(2))
		return &exp, nil
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
	driveResp, err := getDriveEndpoint(fqdn.String()+uri+url, target, retryClient)
	if err != nil {
		log.Error("api call "+fqdn.String()+uri+url+" failed - ", zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		if errors.Is(err, common.ErrInvalidCredential) {
			common.IgnoredDevices[exp.host] = common.IgnoredDevice{
				Name:              exp.host,
				Endpoint:          "https://" + exp.host + "/redfish/v1/Chassis",
				Module:            XL420,
				CredentialProfile: exp.credProfile,
			}
			log.Info("added host "+exp.host+" to ignored list", zap.Any("trace_id", exp.ctx.Value("traceID")))
			var upMetric = (*exp.deviceMetrics)["up"]
			(*upMetric)["up"].WithLabelValues().Set(float64(2))

			return &exp, nil
		}
		return nil, err
	}

	// Loop through Members to get ArrayController URLs
	for _, member := range driveResp.Members {
		// for each ArrayController URL, get the JSON object
		// /redfish/v1/Systems/1/SmartStorage/ArrayControllers/X/
		arrayCtrlResp, err := getDriveEndpoint(fqdn.String()+member.URL, target, retryClient)
		if err != nil {
			log.Error("api call "+fqdn.String()+member.URL+" failed - ", zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
			return nil, err
		}

		// If LogicalDrives is present, parse logical drive endpoint until all urls are found
		if arrayCtrlResp.LinksUpper.LogicalDrives.URL != "" {
			logicalDriveOutput, err := getDriveEndpoint(fqdn.String()+arrayCtrlResp.LinksUpper.LogicalDrives.URL, target, retryClient)
			if err != nil {
				log.Error("api call "+fqdn.String()+arrayCtrlResp.LinksUpper.LogicalDrives.URL+" failed - ", zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
				return nil, err
			}

			// loop through each Member in the "LogicalDrive" field
			for _, member := range logicalDriveOutput.Members {
				// append each URL in the Members array to the logicalDriveURLs array.
				logicalDriveURLs = append(logicalDriveURLs, member.URL)
			}
		}

		// If PhysicalDrives is present, parse physical drive endpoint until all urls are found
		if arrayCtrlResp.LinksUpper.PhysicalDrives.URL != "" {
			physicalDriveOutput, err := getDriveEndpoint(fqdn.String()+arrayCtrlResp.LinksUpper.PhysicalDrives.URL, target, retryClient)
			if err != nil {
				log.Error("api call "+fqdn.String()+arrayCtrlResp.LinksUpper.PhysicalDrives.URL+" failed - ", zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
				return nil, err
			}

			for _, member := range physicalDriveOutput.Members {
				physicalDriveURLs = append(physicalDriveURLs, member.URL)
			}
		}

		// If LogicalDrives is present, parse logical drive endpoint until all urls are found
		if arrayCtrlResp.LinksLower.LogicalDrives.URL != "" {
			logicalDriveOutput, err := getDriveEndpoint(fqdn.String()+arrayCtrlResp.LinksLower.LogicalDrives.URL, target, retryClient)
			if err != nil {
				log.Error("api call "+fqdn.String()+arrayCtrlResp.LinksLower.LogicalDrives.URL+" failed - ", zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
				return nil, err
			}

			// loop through each Member in the "LogicalDrive" field
			for _, member := range logicalDriveOutput.Members {
				// append each URL in the Members array to the logicalDriveURLs array.
				logicalDriveURLs = append(logicalDriveURLs, member.URL)
			}
		}

		// If PhysicalDrives is present, parse physical drive endpoint until all urls are found
		if arrayCtrlResp.LinksLower.PhysicalDrives.URL != "" {
			physicalDriveOutput, err := getDriveEndpoint(fqdn.String()+arrayCtrlResp.LinksLower.PhysicalDrives.URL, target, retryClient)
			if err != nil {
				log.Error("api call "+fqdn.String()+arrayCtrlResp.LinksLower.PhysicalDrives.URL+" failed - ", zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
				return nil, err
			}

			for _, member := range physicalDriveOutput.Members {
				physicalDriveURLs = append(physicalDriveURLs, member.URL)
			}
		}
	}

	// parse to find NVME drives
	chassisOutput, err := getDriveEndpoint(fqdn.String()+uri+chassisUrl, target, retryClient)
	if err != nil {
		log.Error("api call "+fqdn.String()+uri+chassisUrl+" failed - ", zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		return nil, err
	}

	// parse through "Links" to find "Drives" array
	// loop through drives array and append each odata.id url to nvmeDriveURLs list
	for _, drive := range chassisOutput.LinksUpper.Drives {
		nvmeDriveURLs = append(nvmeDriveURLs, drive.URL)
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

	tasks = append(tasks,
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Chassis/1/Thermal/", THERMAL, target, profile, retryClient)),
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Chassis/1/Power/", POWER, target, profile, retryClient)),
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Systems/1/", MEMORY, target, profile, retryClient)),
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Systems/1/", STORAGEBATTERY, target, profile, retryClient)),
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Managers/1/", ILOSELFTEST, target, profile, retryClient)))

	processors, err := getProcessorEndpoints(fqdn.String()+uri+"/Systems/1/Processors/", target, retryClient)
	if err != nil {
		log.Error("error when getting Processors endpoints from "+XL420, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		return nil, err
	}

	for _, processor := range processors.Members {
		tasks = append(tasks,
			pool.NewTask(common.Fetch(fqdn.String()+processor.URL, PROCESSOR, target, profile, retryClient)))
	}

	exp.pool = pool.NewPool(tasks, 1)

	return &exp, nil
}

// Describe describes all the metrics ever exported by the fishymetrics exporter. It
// implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range *e.deviceMetrics {
		for _, n := range *m {
			n.Describe(ch)
		}
	}
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
		var upMetric = (*e.deviceMetrics)["up"]
		(*upMetric)["up"].WithLabelValues().Set(float64(2))
	}

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
			if errors.Is(task.Err, common.ErrInvalidCredential) {
				common.IgnoredDevices[e.host] = common.IgnoredDevice{
					Name:              e.host,
					Endpoint:          "https://" + e.host + "/redfish/v1/Chassis",
					Module:            XL420,
					CredentialProfile: e.credProfile,
				}
				log.Info("added host "+e.host+" to ignored list", zap.Any("trace_id", e.ctx.Value("traceID")))
				deviceState = 2
			} else {
				deviceState = 0
			}
			var upMetric = (*e.deviceMetrics)["up"]
			(*upMetric)["up"].WithLabelValues().Set(float64(deviceState))
			log.Error("error from "+XL420, zap.Error(task.Err), zap.String("api", task.MetricType), zap.Any("trace_id", e.ctx.Value("traceID")))
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
		case PROCESSOR:
			err = e.exportProcessorMetrics(task.Body)
		case STORAGEBATTERY:
			err = e.exportStorageBattery(task.Body)
		case ILOSELFTEST:
			err = e.exportIloSelfTest(task.Body)
		}

		if err != nil {
			log.Error("error exporting metrics - from "+XL420, zap.Error(err), zap.String("api", task.MetricType), zap.Any("trace_id", e.ctx.Value("traceID")))
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

	var upMetric = (*e.deviceMetrics)["up"]
	(*upMetric)["up"].WithLabelValues().Set(float64(state))

}

// exportPowerMetrics collects the XL420's power metrics in json format and sets the prometheus gauges
func (e *Exporter) exportPowerMetrics(body []byte) error {

	var state float64
	var pm PowerMetrics
	var dlPower = (*e.deviceMetrics)["powerMetrics"]
	err := json.Unmarshal(body, &pm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling XL420 PowerMetrics - " + err.Error())
	}

	for idx, pc := range pm.PowerControl {
		if pc.MemberID != "" {
			(*dlPower)["supplyTotalConsumed"].WithLabelValues(pc.MemberID).Set(float64(pc.PowerConsumedWatts))
			(*dlPower)["supplyTotalCapacity"].WithLabelValues(pc.MemberID).Set(float64(pc.PowerCapacityWatts))
		} else {
			(*dlPower)["supplyTotalConsumed"].WithLabelValues(strconv.Itoa(idx)).Set(float64(pc.PowerConsumedWatts))
			(*dlPower)["supplyTotalCapacity"].WithLabelValues(strconv.Itoa(idx)).Set(float64(pc.PowerCapacityWatts))
		}
	}

	for _, ps := range pm.PowerSupplies {
		if ps.Status.State == "Enabled" {
			if ps.MemberID != "" {
				(*dlPower)["supplyOutput"].WithLabelValues(ps.MemberID, ps.SparePartNumber).Set(float64(ps.LastPowerOutputWatts))
			} else {
				(*dlPower)["supplyOutput"].WithLabelValues(strconv.Itoa(ps.Oem.Hp.BayNumber), ps.SparePartNumber).Set(float64(ps.LastPowerOutputWatts))
			}
			if ps.Status.Health == "OK" {
				state = OK
			} else {
				state = BAD
			}
			if ps.MemberID != "" {
				(*dlPower)["supplyStatus"].WithLabelValues(ps.MemberID, ps.SparePartNumber).Set(state)
			} else {
				(*dlPower)["supplyStatus"].WithLabelValues(strconv.Itoa(ps.Oem.Hp.BayNumber), ps.SparePartNumber).Set(state)
			}
		}
	}

	return nil
}

// exportThermalMetrics collects the XL420's thermal and fan metrics in json format and sets the prometheus gauges
func (e *Exporter) exportThermalMetrics(body []byte) error {

	var state float64
	var tm ThermalMetrics
	var dlThermal = (*e.deviceMetrics)["thermalMetrics"]
	err := json.Unmarshal(body, &tm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling XL420 ThermalMetrics - " + err.Error())
	}

	// Iterate through fans
	for _, fan := range tm.Fans {
		// Check fan status and convert string to numeric values
		if fan.Status.State == "Enabled" {
			if fan.FanName != "" {
				(*dlThermal)["fanSpeed"].WithLabelValues(fan.FanName).Set(float64(fan.CurrentReading))
			} else {
				(*dlThermal)["fanSpeed"].WithLabelValues(fan.Name).Set(float64(fan.Reading))
			}
			if fan.Status.Health == "OK" {
				state = OK
			} else {
				state = BAD
			}
			if fan.FanName != "" {
				(*dlThermal)["fanStatus"].WithLabelValues(fan.FanName).Set(state)
			} else {
				(*dlThermal)["fanStatus"].WithLabelValues(fan.Name).Set(state)
			}
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

// exportLogicalDriveMetrics collects the DL560 logical drive metrics in json format and sets the prometheus gauges
func (e *Exporter) exportLogicalDriveMetrics(body []byte) error {

	var state float64
	var dld LogicalDriveMetrics
	var dlDrive = (*e.deviceMetrics)["logicalDriveMetrics"]
	err := json.Unmarshal(body, &dld)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling XL420 LogicalDriveMetrics - " + err.Error())
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

	(*dlDrive)["raidStatus"].WithLabelValues(dld.Name, dld.LogicalDriveName, dld.VolumeUniqueIdentifier, dld.Raid).Set(state)

	return nil
}

// exportPhysicalDriveMetrics collects the XL420 drive metrics in json format and sets the prometheus gauges
func (e *Exporter) exportPhysicalDriveMetrics(body []byte) error {

	var state float64
	var dpd DiskDriveMetrics
	var dpDrive = (*e.deviceMetrics)["diskDriveMetrics"]
	err := json.Unmarshal(body, &dpd)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling XL420 DriveMetrics - " + err.Error())
	}
	// Check physical drive is enabled then check status and convert string to numeric values
	if dpd.Status.State == "Enabled" {
		if dpd.Status.Health == "OK" {
			state = OK
		} else {
			state = BAD
		}
	} else {
		state = DISABLED
	}

	(*dpDrive)["driveStatus"].WithLabelValues(dpd.Name, dpd.Id, dpd.Location, dpd.SerialNumber).Set(state)

	return nil
}

// exportNVMeDriveMetrics collects the XL420 NVME drive metrics in json format and sets the prometheus gauges
func (e *Exporter) exportNVMeDriveMetrics(body []byte) error {
	var state float64
	var dlnvme NVMeDriveMetrics
	var dlnvmedrive = (*e.deviceMetrics)["nvmeMetrics"]
	err := json.Unmarshal(body, &dlnvme)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling XL420 NVMeDriveMetrics - " + err.Error())
	}

	// Check nvme drive is enabled then check status and convert string to numeric values
	if dlnvme.Oem.Hpe.DriveStatus.State == "Enabled" {
		if dlnvme.Oem.Hpe.DriveStatus.Health == "OK" {
			state = OK
		} else {
			state = BAD
		}
	} else {
		state = DISABLED
	}

	(*dlnvmedrive)["nvmeDriveStatus"].WithLabelValues(dlnvme.Protocol, dlnvme.ID, dlnvme.PhysicalLocation.PartLocation.ServiceLabel).Set(state)
	return nil
}

// exportStorageBattery collects the XL420's smart storge battery metrics in json format and sets the prometheus guage
func (e *Exporter) exportStorageBattery(body []byte) error {

	var state float64
	var sysm SystemMetrics
	var storBattery = (*e.deviceMetrics)["storBatteryMetrics"]
	err := json.Unmarshal(body, &sysm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling XL420 Storage Battery Metrics - " + err.Error())
	}

	if fmt.Sprint(sysm.Oem.Hp.Battery) != "null" && len(sysm.Oem.Hp.Battery) > 0 {
		for _, ssbat := range sysm.Oem.Hp.Battery {
			if ssbat.Present == "Yes" {
				if ssbat.Condition == "Ok" {
					state = OK
				} else {
					state = BAD
				}
				(*storBattery)["storageBatteryStatus"].WithLabelValues(strconv.Itoa(ssbat.Index), ssbat.Name, ssbat.Model, ssbat.SerialNumber).Set(state)
			}
		}
	} else if fmt.Sprint(sysm.Oem.Hpe.Battery) != "null" && len(sysm.Oem.Hpe.Battery) > 0 {
		for _, ssbat := range sysm.Oem.Hpe.Battery {
			if ssbat.Present == "Yes" {
				if ssbat.Condition == "Ok" {
					state = OK
				} else {
					state = BAD
				}
				(*storBattery)["storageBatteryStatus"].WithLabelValues(strconv.Itoa(ssbat.Index), ssbat.Name, ssbat.Model, ssbat.SerialNumber).Set(state)
			}
		}
	}

	return nil
}

// exportMemoryMetrics collects the XL420 drive metrics in json format and sets the prometheus gauges
func (e *Exporter) exportMemoryMetrics(body []byte) error {

	var state float64
	var dlm MemoryMetrics
	var dlMemory = (*e.deviceMetrics)["memoryMetrics"]
	err := json.Unmarshal(body, &dlm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling XL420 MemoryMetrics - " + err.Error())
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

// exportProcessorMetrics collects the XL420 processor metrics in json format and sets the prometheus gauges
func (e *Exporter) exportProcessorMetrics(body []byte) error {

	var state float64
	var pm ProcessorMetrics
	var proc = (*e.deviceMetrics)["processorMetrics"]
	err := json.Unmarshal(body, &pm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling XL420 ProcessorMetrics - " + err.Error())
	}

	if pm.Status.Health == "OK" {
		state = OK
	} else {
		state = BAD
	}
	(*proc)["processorStatus"].WithLabelValues(pm.Id, pm.Socket, pm.Model, strconv.Itoa(pm.TotalCores)).Set(state)

	return nil
}

// exportIloSelfTest collects the XL420's iLO Self Test Results metrics in json format and sets the prometheus guage
func (e *Exporter) exportIloSelfTest(body []byte) error {

	var state float64
	var sysm SystemMetrics
	var iloSelfTst = (*e.deviceMetrics)["iloSelfTestMetrics"]
	err := json.Unmarshal(body, &sysm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling XL420 iLO Self Test Metrics - " + err.Error())
	}

	if fmt.Sprint(sysm.Oem.Hp.IloSelfTest) != "null" && len(sysm.Oem.Hp.IloSelfTest) > 0 {
		for _, ilost := range sysm.Oem.Hp.IloSelfTest {
			if ilost.Status != "Informational" {
				if ilost.Status == "OK" {
					state = OK
				} else {
					state = BAD
				}
				(*iloSelfTst)["iloSelfTestStatus"].WithLabelValues(ilost.Name).Set(state)
			}
		}
	} else if fmt.Sprint(sysm.Oem.Hpe.IloSelfTest) != "null" && len(sysm.Oem.Hpe.IloSelfTest) > 0 {
		for _, ilost := range sysm.Oem.Hpe.IloSelfTest {
			if ilost.Status != "Informational" {
				if ilost.Status == "OK" {
					state = OK
				} else {
					state = BAD
				}
				(*iloSelfTst)["iloSelfTestStatus"].WithLabelValues(ilost.Name).Set(state)
			}
		}
	}

	return nil
}

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
		} else if resp.StatusCode == http.StatusUnauthorized {
			return drive, common.ErrInvalidCredential
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
		return drive, fmt.Errorf("Error Unmarshalling XL420 Drive Collection struct - " + err.Error())
	}

	return drive, nil
}

func getProcessorEndpoints(url, host string, client *retryablehttp.Client) (Collection, error) {
	var processors Collection
	var resp *http.Response
	var err error
	retryCount := 0
	req := common.BuildRequest(url, host)

	resp, err = common.DoRequest(client, req)
	if err != nil {
		return processors, err
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
				return processors, err
			} else if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
				return processors, fmt.Errorf("HTTP status %d", resp.StatusCode)
			}
		} else if resp.StatusCode == http.StatusUnauthorized {
			return processors, common.ErrInvalidCredential
		} else {
			return processors, fmt.Errorf("HTTP status %d", resp.StatusCode)
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return processors, fmt.Errorf("Error reading Response Body - " + err.Error())
	}

	err = json.Unmarshal(body, &processors)
	if err != nil {
		return processors, fmt.Errorf("Error Unmarshalling XL420 Processors Collection struct - " + err.Error())
	}

	return processors, nil
}
