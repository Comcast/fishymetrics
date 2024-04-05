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
	"github.com/comcast/fishymetrics/oem"
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
	// MEMORY_SUMMARY represents the memory metric endpoints
	MEMORY_SUMMARY = "MemorySummaryMetrics"
	// FIRMWARE represents the firmware metric endpoints
	FIRMWARE = "FirmwareMetrics"
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
	ctx                 context.Context
	mutex               sync.RWMutex
	pool                *pool.Pool
	host                string
	credProfile         string
	biosVersion         string
	chassisSerialNumber string
	iloServerName       string
	deviceMetrics       *map[string]*metrics
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

	// chassis system endpoint to use for memory, processor, bios version scrapes
	sysEndpoint, err := getChassisEndpoint(fqdn.String()+uri+"/Managers/1/", target, retryClient)
	if err != nil {
		log.Error("error when getting chassis endpoint from "+XL420, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
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

	resp, err := getSystemsMetadata(fqdn.String()+sysEndpoint, target, retryClient)
	if err != nil {
		log.Error("error when getting BIOS version from "+XL420, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		return nil, err
	}
	exp.biosVersion = resp.BiosVersion
	exp.chassisSerialNumber = resp.SerialNumber
	exp.iloServerName = resp.IloServerName

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

	// DIMM endpoints array
	dimms, err := getDIMMEndpoints(fqdn.String()+sysEndpoint+"Memory/", target, retryClient)
	if err != nil {
		log.Error("error when getting DIMM endpoints from "+XL420, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		return nil, err
	}

	tasks = append(tasks,
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Chassis/1/Thermal/", THERMAL, target, profile, retryClient)),
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Chassis/1/Power/", POWER, target, profile, retryClient)),
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Systems/1/", MEMORY_SUMMARY, target, profile, retryClient)),
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Managers/1/", FIRMWARE, target, profile, retryClient)))

	// DIMMs
	for _, dimm := range dimms.Members {
		tasks = append(tasks,
			pool.NewTask(common.Fetch(fqdn.String()+dimm.URL, MEMORY, target, profile, retryClient)))
	}

	tasks = append(tasks,
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
		case FIRMWARE:
			err = e.exportFirmwareMetrics(task.Body)
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
		case MEMORY_SUMMARY:
			err = e.exportMemorySummaryMetrics(task.Body)
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

// exportFirmwareMetrics collects the device metrics in json format and sets the prometheus gauges
func (e *Exporter) exportFirmwareMetrics(body []byte) error {
	var chas oem.Chassis
	var dm = (*e.deviceMetrics)["deviceInfo"]
	err := json.Unmarshal(body, &chas)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling XL420 FirmwareMetrics - " + err.Error())
	}

	(*dm)["deviceInfo"].WithLabelValues(e.iloServerName, e.chassisSerialNumber, chas.FirmwareVersion, e.biosVersion, XL420).Set(1.0)

	return nil
}

// exportPowerMetrics collects the power metrics in json format and sets the prometheus gauges
func (e *Exporter) exportPowerMetrics(body []byte) error {

	var state float64
	var pm oem.PowerMetrics
	var pow = (*e.deviceMetrics)["powerMetrics"]
	err := json.Unmarshal(body, &pm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling XL420 PowerMetrics - " + err.Error())
	}

	for _, pc := range pm.PowerControl.PowerControl {
		var watts float64
		switch pc.PowerConsumedWatts.(type) {
		case float64:
			if pc.PowerConsumedWatts.(float64) > 0 {
				watts = pc.PowerConsumedWatts.(float64)
			}
		case string:
			if pc.PowerConsumedWatts.(string) != "" {
				watts, _ = strconv.ParseFloat(pc.PowerConsumedWatts.(string), 32)
			}
		default:
			// use the AverageConsumedWatts if PowerConsumedWatts is not present
			switch pc.PowerMetrics.AverageConsumedWatts.(type) {
			case float64:
				watts = pc.PowerMetrics.AverageConsumedWatts.(float64)
			case string:
				watts, _ = strconv.ParseFloat(pc.PowerMetrics.AverageConsumedWatts.(string), 32)
			}
		}
		(*pow)["supplyTotalConsumed"].WithLabelValues(pc.MemberID, e.chassisSerialNumber).Set(watts)
	}

	for _, pv := range pm.Voltages {
		if pv.Status.State == "Enabled" {
			var volts float64
			switch pv.ReadingVolts.(type) {
			case float64:
				volts = pv.ReadingVolts.(float64)
			case string:
				volts, _ = strconv.ParseFloat(pv.ReadingVolts.(string), 32)
			}
			(*pow)["voltageOutput"].WithLabelValues(pv.Name, e.chassisSerialNumber).Set(volts)
			if pv.Status.Health == "OK" {
				state = OK
			} else {
				state = BAD
			}
		} else {
			state = BAD
		}

		(*pow)["voltageStatus"].WithLabelValues(pv.Name, e.chassisSerialNumber).Set(state)
	}

	for _, ps := range pm.PowerSupplies {
		if ps.Status.State == "Enabled" {
			var watts float64
			switch ps.LastPowerOutputWatts.(type) {
			case float64:
				watts = ps.LastPowerOutputWatts.(float64)
			case string:
				watts, _ = strconv.ParseFloat(ps.LastPowerOutputWatts.(string), 32)
			}
			(*pow)["supplyOutput"].WithLabelValues(ps.Name, e.chassisSerialNumber, ps.Manufacturer, ps.SparePartNumber, ps.SerialNumber, ps.PowerSupplyType, ps.Model).Set(watts)
			if ps.Status.Health == "OK" {
				state = OK
			} else if ps.Status.Health == "" {
				state = OK
			} else {
				state = BAD
			}
		} else {
			state = BAD
		}

		(*pow)["supplyStatus"].WithLabelValues(ps.Name, e.chassisSerialNumber, ps.Manufacturer, ps.SparePartNumber, ps.SerialNumber, ps.PowerSupplyType, ps.Model).Set(state)
	}

	return nil
}

// exportThermalMetrics collects the thermal and fan metrics in json format and sets the prometheus gauges
func (e *Exporter) exportThermalMetrics(body []byte) error {

	var state float64
	var tm oem.ThermalMetrics
	var therm = (*e.deviceMetrics)["thermalMetrics"]
	err := json.Unmarshal(body, &tm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling XL420 ThermalMetrics - " + err.Error())
	}

	// Iterate through fans
	for _, fan := range tm.Fans {
		// Check fan status and convert string to numeric values
		if fan.Status.State == "Enabled" {
			var fanSpeed float64
			switch fan.Reading.(type) {
			case string:
				fanSpeed, _ = strconv.ParseFloat(fan.Reading.(string), 32)
			case float64:
				fanSpeed = fan.Reading.(float64)
			}

			if fan.FanName != "" {
				(*therm)["fanSpeed"].WithLabelValues(fan.FanName, e.chassisSerialNumber).Set(float64(fan.CurrentReading))
			} else {
				(*therm)["fanSpeed"].WithLabelValues(fan.Name, e.chassisSerialNumber).Set(fanSpeed)
			}
			if fan.Status.Health == "OK" {
				state = OK
			} else {
				state = BAD
			}
			if fan.FanName != "" {
				(*therm)["fanStatus"].WithLabelValues(fan.FanName, e.chassisSerialNumber).Set(state)
			} else {
				(*therm)["fanStatus"].WithLabelValues(fan.Name, e.chassisSerialNumber).Set(state)
			}
		}
	}

	// Iterate through sensors
	for _, sensor := range tm.Temperatures {
		// Check sensor status and convert string to numeric values
		if sensor.Status.State == "Enabled" {
			var celsTemp float64
			switch sensor.ReadingCelsius.(type) {
			case string:
				celsTemp, _ = strconv.ParseFloat(sensor.ReadingCelsius.(string), 32)
			case int:
				celsTemp = float64(sensor.ReadingCelsius.(int))
			case float64:
				celsTemp = sensor.ReadingCelsius.(float64)
			}
			(*therm)["sensorTemperature"].WithLabelValues(strings.TrimRight(sensor.Name, " "), e.chassisSerialNumber).Set(celsTemp)
			if sensor.Status.Health == "OK" {
				state = OK
			} else {
				state = BAD
			}
			(*therm)["sensorStatus"].WithLabelValues(strings.TrimRight(sensor.Name, " "), e.chassisSerialNumber).Set(state)
		}
	}

	return nil
}

// exportPhysicalDriveMetrics collects the physical drive metrics in json format and sets the prometheus gauges
func (e *Exporter) exportPhysicalDriveMetrics(body []byte) error {

	var state float64
	var dlphysical oem.DiskDriveMetrics
	var dlphysicaldrive = (*e.deviceMetrics)["diskDriveMetrics"]
	err := json.Unmarshal(body, &dlphysical)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling XL420 DiskDriveMetrics - " + err.Error())
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

	// Physical drives need to have a unique identifier like location so as to not overwrite data
	// physical drives can have the same ID, but belong to a different ArrayController, therefore need more than just the ID as a unique identifier.
	(*dlphysicaldrive)["driveStatus"].WithLabelValues(dlphysical.Name, e.chassisSerialNumber, dlphysical.Id, dlphysical.Location, dlphysical.SerialNumber).Set(state)
	return nil
}

// exportLogicalDriveMetrics collects the physical drive metrics in json format and sets the prometheus gauges
func (e *Exporter) exportLogicalDriveMetrics(body []byte) error {
	var state float64
	var dllogical oem.LogicalDriveMetrics
	var dllogicaldrive = (*e.deviceMetrics)["logicalDriveMetrics"]
	err := json.Unmarshal(body, &dllogical)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling XL420 LogicalDriveMetrics - " + err.Error())
	}
	// Check physical drive is enabled then check status and convert string to numeric values
	if dllogical.Status.State == "Enabled" {
		if dllogical.Status.Health == "OK" {
			state = OK
		} else {
			state = BAD
		}
	} else {
		state = DISABLED
	}

	(*dllogicaldrive)["raidStatus"].WithLabelValues(dllogical.Name, e.chassisSerialNumber, dllogical.LogicalDriveName, dllogical.VolumeUniqueIdentifier, dllogical.Raid).Set(state)
	return nil
}

// exportNVMeDriveMetrics collects the XL420 NVME drive metrics in json format and sets the prometheus gauges
func (e *Exporter) exportNVMeDriveMetrics(body []byte) error {
	var state float64
	var dlnvme oem.NVMeDriveMetrics
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

	(*dlnvmedrive)["nvmeDriveStatus"].WithLabelValues(e.chassisSerialNumber, dlnvme.Protocol, dlnvme.ID, dlnvme.PhysicalLocation.PartLocation.ServiceLabel).Set(state)
	return nil
}

// exportStorageBattery collects the XL420's smart storge battery metrics in json format and sets the prometheus guage
func (e *Exporter) exportStorageBattery(body []byte) error {

	var state float64
	var sysm oem.SystemMetrics
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
				(*storBattery)["storageBatteryStatus"].WithLabelValues(strconv.Itoa(ssbat.Index), e.chassisSerialNumber, ssbat.Name, ssbat.Model, ssbat.SerialNumber).Set(state)
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
				(*storBattery)["storageBatteryStatus"].WithLabelValues(strconv.Itoa(ssbat.Index), e.chassisSerialNumber, ssbat.Name, ssbat.Model, ssbat.SerialNumber).Set(state)
			}
		}
	}

	return nil
}

// exportMemorySummaryMetrics collects the memory summary metrics in json format and sets the prometheus gauges
func (e *Exporter) exportMemorySummaryMetrics(body []byte) error {

	var state float64
	var dlm oem.MemorySummaryMetrics
	var dlMemory = (*e.deviceMetrics)["memoryMetrics"]
	err := json.Unmarshal(body, &dlm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling XL420 MemorySummaryMetrics - " + err.Error())
	}
	// Check memory status and convert string to numeric values
	if dlm.MemorySummary.Status.HealthRollup == "OK" {
		state = OK
	} else {
		state = BAD
	}

	(*dlMemory)["memoryStatus"].WithLabelValues(e.chassisSerialNumber, strconv.Itoa(dlm.MemorySummary.TotalSystemMemoryGiB)).Set(state)

	return nil
}

// exportMemoryMetrics collects the memory dimm metrics in json format and sets the prometheus gauges
func (e *Exporter) exportMemoryMetrics(body []byte) error {

	var state float64
	var memCap string
	var mm oem.MemoryMetrics
	var mem = (*e.deviceMetrics)["memoryMetrics"]
	err := json.Unmarshal(body, &mm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling XL420 MemoryMetrics - " + err.Error())
	}

	if mm.DIMMStatus != "" {
		switch mm.SizeMB.(type) {
		case string:
			memCap = mm.SizeMB.(string)
		case int:
			memCap = strconv.Itoa(mm.SizeMB.(int))
		case float64:
			memCap = strconv.Itoa(int(mm.SizeMB.(float64)))
		}
		if mm.DIMMStatus == "GoodInUse" {
			state = OK
		} else {
			state = BAD
		}
		(*mem)["memoryDimmStatus"].WithLabelValues(mm.Name, e.chassisSerialNumber, memCap, strings.TrimRight(mm.Manufacturer, " "), strings.TrimRight(mm.PartNumber, " "), mm.SerialNumber).Set(state)
	} else if mm.Status != "" {
		var status string

		switch mm.CapacityMiB.(type) {
		case string:
			memCap = mm.CapacityMiB.(string)
		case int:
			memCap = strconv.Itoa(mm.CapacityMiB.(int))
		case float64:
			memCap = strconv.Itoa(int(mm.CapacityMiB.(float64)))
		}

		switch mm.Status.(type) {
		case string:
			status = mm.Status.(string)
			if status == "Operable" {
				state = OK
			} else {
				state = BAD
			}
		default:
			if s, ok := mm.Status.(map[string]interface{}); ok {
				switch s["State"].(type) {
				case string:
					if s["State"].(string) == "Enabled" {
						switch s["Health"].(type) {
						case string:
							if s["Health"].(string) == "OK" {
								state = OK
							} else if s["Health"].(string) == "" {
								state = OK
							} else {
								state = BAD
							}
						case nil:
							state = OK
						}
					} else if s["State"].(string) == "Absent" {
						return nil
					} else {
						state = BAD
					}
				}
			}
		}
		(*mem)["memoryDimmStatus"].WithLabelValues(mm.Name, e.chassisSerialNumber, memCap, mm.Manufacturer, strings.TrimRight(mm.PartNumber, " "), mm.SerialNumber).Set(state)
	}

	return nil
}

// exportProcessorMetrics collects the XL420 processor metrics in json format and sets the prometheus gauges
func (e *Exporter) exportProcessorMetrics(body []byte) error {

	var state float64
	var totCores string
	var pm oem.ProcessorMetrics
	var proc = (*e.deviceMetrics)["processorMetrics"]
	err := json.Unmarshal(body, &pm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling XL420 ProcessorMetrics - " + err.Error())
	}

	switch pm.TotalCores.(type) {
	case string:
		totCores = pm.TotalCores.(string)
	case float64:
		totCores = strconv.Itoa(int(pm.TotalCores.(float64)))
	case int:
		totCores = strconv.Itoa(pm.TotalCores.(int))
	}
	if pm.Status.Health == "OK" {
		state = OK
	} else {
		state = BAD
	}
	(*proc)["processorStatus"].WithLabelValues(pm.Id, e.chassisSerialNumber, pm.Socket, pm.Model, totCores).Set(state)

	return nil
}

// exportIloSelfTest collects the XL420's iLO Self Test Results metrics in json format and sets the prometheus guage
func (e *Exporter) exportIloSelfTest(body []byte) error {

	var state float64
	var sysm oem.SystemMetrics
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
				(*iloSelfTst)["iloSelfTestStatus"].WithLabelValues(ilost.Name, e.chassisSerialNumber).Set(state)
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
				(*iloSelfTst)["iloSelfTestStatus"].WithLabelValues(ilost.Name, e.chassisSerialNumber).Set(state)
			}
		}
	}

	return nil
}

func getChassisEndpoint(url, host string, client *retryablehttp.Client) (string, error) {
	var chas oem.Chassis
	var urlFinal string
	req := common.BuildRequest(url, host)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
		if resp.StatusCode == http.StatusUnauthorized {
			return "", common.ErrInvalidCredential
		} else {
			return "", fmt.Errorf("HTTP status %d", resp.StatusCode)
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Error reading Response Body - " + err.Error())
	}

	err = json.Unmarshal(body, &chas)
	if err != nil {
		return "", fmt.Errorf("Error Unmarshalling XL420 Chassis struct - " + err.Error())
	}

	if len(chas.LinksUpper.ManagerForServers.ServerManagerURLSlice) > 0 {
		urlFinal = chas.LinksUpper.ManagerForServers.ServerManagerURLSlice[0]
	} else if len(chas.LinksLower.ManagerForServers.ServerManagerURLSlice) > 0 {
		urlFinal = chas.LinksLower.ManagerForServers.ServerManagerURLSlice[0]
	}

	return urlFinal, nil
}

func getSystemsMetadata(url, host string, client *retryablehttp.Client) (oem.ServerManager, error) {
	var sm oem.ServerManager
	req := common.BuildRequest(url, host)

	resp, err := client.Do(req)
	if err != nil {
		return sm, err
	}
	defer resp.Body.Close()
	if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
		return sm, fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return sm, fmt.Errorf("Error reading Response Body - " + err.Error())
	}

	err = json.Unmarshal(body, &sm)
	if err != nil {
		return sm, fmt.Errorf("Error Unmarshalling XL420 ServerManager struct - " + err.Error())
	}

	return sm, nil
}

func getDIMMEndpoints(url, host string, client *retryablehttp.Client) (oem.Collection, error) {
	var dimms oem.Collection
	var resp *http.Response
	var err error
	retryCount := 0
	req := common.BuildRequest(url, host)

	resp, err = common.DoRequest(client, req)
	if err != nil {
		return dimms, err
	}
	defer resp.Body.Close()
	if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
		if resp.StatusCode == http.StatusNotFound {
			for retryCount < 1 && resp.StatusCode == http.StatusNotFound {
				time.Sleep(client.RetryWaitMin)
				resp, err = common.DoRequest(client, req)
				retryCount = retryCount + 1
			}
			if err != nil {
				return dimms, err
			} else if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
				return dimms, fmt.Errorf("HTTP status %d", resp.StatusCode)
			}
		} else {
			return dimms, fmt.Errorf("HTTP status %d", resp.StatusCode)
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return dimms, fmt.Errorf("Error reading Response Body - " + err.Error())
	}

	err = json.Unmarshal(body, &dimms)
	if err != nil {
		return dimms, fmt.Errorf("Error Unmarshalling XL420 Memory Collection struct - " + err.Error())
	}

	return dimms, nil
}

func getDriveEndpoint(url, host string, client *retryablehttp.Client) (oem.GenericDrive, error) {
	var drive oem.GenericDrive
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

func getProcessorEndpoints(url, host string, client *retryablehttp.Client) (oem.Collection, error) {
	var processors oem.Collection
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
