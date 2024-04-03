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

package s3260m4

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
	"path"
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
	// S3260M4 is a Cisco Hardware Device we scrape
	S3260M4 = "s3260m4"
	// THERMAL represents the thermal metric endpoint
	THERMAL = "ThermalMetrics"
	// POWER represents the power metric endpoint
	POWER = "PowerMetrics"
	// DRIVE represents the logical drive metric endpoints
	DRIVE = "DriveMetrics"
	// MEMORY represents the memory metric endpoints
	MEMORY = "MemoryMetrics"
	// PROCESSOR represents the processor metric endpoints
	PROCESSOR = "ProcessorMetrics"
	// FIRMWARE represents the firmware metric endpoints
	FIRMWARE = "FirmwareMetrics"
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
	deviceMetrics       *map[string]*metrics
}

// NewExporter returns an initialized Exporter for Cisco UCS S3260M4 device.
func NewExporter(ctx context.Context, target, uri, profile string) (*Exporter, error) {
	var fqdn *url.URL
	var tasks []*pool.Task
	var exp = Exporter{
		ctx:           ctx,
		credProfile:   profile,
		deviceMetrics: NewDeviceMetrics(),
	}
	var mgr string

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
	retryClient.HTTPClient.Timeout = 1 * time.Minute
	retryClient.Logger = nil
	retryClient.RetryWaitMin = 1 * time.Second
	retryClient.RetryWaitMax = 1 * time.Second
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
	mgrEndpoints, err := getManagerEndpoint(fqdn.String()+uri+"/Managers/BMC2", target, retryClient)
	if err != nil {
		log.Error("error when getting managers endpoint from "+S3260M4, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		if errors.Is(err, common.ErrInvalidCredential) {
			common.IgnoredDevices[exp.host] = common.IgnoredDevice{
				Name:              exp.host,
				Endpoint:          "https://" + exp.host + "/redfish/v1/Chassis",
				Module:            S3260M4,
				CredentialProfile: exp.credProfile,
			}
			log.Info("added host "+exp.host+" to ignored list", zap.Any("trace_id", exp.ctx.Value("traceID")))
			var upMetric = (*exp.deviceMetrics)["up"]
			(*upMetric)["up"].WithLabelValues().Set(float64(2))

			return &exp, nil
		}
		return nil, err
	}

	if len(mgrEndpoints.Links.ManagerForServers.ServerManagerURLSlice) > 0 {
		mgr = mgrEndpoints.Links.ManagerForServers.ServerManagerURLSlice[0]
	}

	// chassis BIOS version
	biosVer, err := getBIOSVersion(fqdn.String()+mgr, target, retryClient)
	if err != nil {
		log.Error("error when getting BIOS version from "+S3260M4, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		return nil, err
	}
	exp.biosVersion = biosVer

	// chassis serial number
	chassisSN, err := getChassisSerialNumber(fqdn.String()+uri+"/Chassis/CMC", target, retryClient)
	if err != nil {
		log.Error("error when getting chassis serial number from "+S3260M4, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		return nil, err
	}
	exp.chassisSerialNumber = chassisSN

	// DIMM endpoints array
	dimms, err := getDIMMEndpoints(fqdn.String()+mgr+"/Memory", target, retryClient)
	if err != nil {
		log.Error("error when getting DIMM endpoints from "+S3260M4, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		return nil, err
	}

	// chassis CMC/Server1/Server2 endpoints
	chass, err := getChassisEndpoint(fqdn.String()+uri+"/Chassis", target, retryClient)
	if err != nil {
		log.Error("error when getting chassis endpoint from "+S3260M4, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		return nil, err
	}

	serial := path.Base(mgr)

	tasks = append(tasks,
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Managers/CIMC", FIRMWARE, target, profile, retryClient)))

	for _, ch := range chass.Members {
		tasks = append(tasks,
			pool.NewTask(common.Fetch(fqdn.String()+ch.URL+"/Thermal", THERMAL, target, profile, retryClient)),
			pool.NewTask(common.Fetch(fqdn.String()+ch.URL+"/Power", POWER, target, profile, retryClient)),
		)
	}

	rcontrollers, err := getRaidEndpoint(fqdn.String()+uri+"/Systems/"+serial+"/Storage", target, retryClient)
	if err != nil {
		log.Error("error when getting storage controller endpoints from "+S3260M4, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		return nil, err
	}

	for _, rcontroller := range rcontrollers.Members {
		tasks = append(tasks,
			pool.NewTask(common.Fetch(fqdn.String()+rcontroller.URL, DRIVE, target, profile, retryClient)))
	}

	tasks = append(tasks,
		pool.NewTask(common.Fetch(fqdn.String()+mgr+"/Processors/CPU1", PROCESSOR, target, profile, retryClient)),
		pool.NewTask(common.Fetch(fqdn.String()+mgr+"/Processors/CPU2", PROCESSOR, target, profile, retryClient)))

	for _, dimm := range dimms.Members {
		tasks = append(tasks,
			pool.NewTask(common.Fetch(fqdn.String()+dimm.URL, MEMORY, target, profile, retryClient)))
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
					Module:            S3260M4,
					CredentialProfile: e.credProfile,
				}
				log.Info("added host "+e.host+" to ignored list", zap.Any("trace_id", e.ctx.Value("traceID")))
				deviceState = 2
			} else {
				deviceState = 0
			}
			var upMetric = (*e.deviceMetrics)["up"]
			(*upMetric)["up"].WithLabelValues().Set(float64(deviceState))
			log.Error("error from "+S3260M4, zap.Error(task.Err), zap.String("api", task.MetricType), zap.Any("trace_id", e.ctx.Value("traceID")))
			return
		}

		switch task.MetricType {
		case FIRMWARE:
			err = e.exportFirmwareMetrics(task.Body)
		case THERMAL:
			err = e.exportThermalMetrics(task.Body)
		case POWER:
			err = e.exportPowerMetrics(task.Body)
		case MEMORY:
			err = e.exportMemoryMetrics(task.Body)
		case PROCESSOR:
			err = e.exportProcessorMetrics(task.Body)
		case DRIVE:
			err = e.exportDriveMetrics(task.Body)
		}

		if err != nil {
			scrapeChan <- 0
			log.Error("error exporting metrics - from "+S3260M4, zap.Error(err), zap.String("api", task.MetricType), zap.Any("trace_id", e.ctx.Value("traceID")))
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

// exportFirmwareMetrics collects the Cisco UCS S3260M4's device metrics in json format and sets the prometheus gauges
func (e *Exporter) exportFirmwareMetrics(body []byte) error {
	var chas oem.Chassis
	var dm = (*e.deviceMetrics)["deviceInfo"]
	err := json.Unmarshal(body, &chas)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling S3260M4 FirmwareMetrics - " + err.Error())
	}

	(*dm)["deviceInfo"].WithLabelValues(chas.Description, e.chassisSerialNumber, chas.FirmwareVersion, e.biosVersion, S3260M4).Set(1.0)

	return nil
}

// exportPowerMetrics collects the Cisco UCS S3260M4's power metrics in json format and sets the prometheus gauges
func (e *Exporter) exportPowerMetrics(body []byte) error {

	var state float64
	var pm oem.PowerMetrics
	var pow = (*e.deviceMetrics)["powerMetrics"]
	err := json.Unmarshal(body, &pm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling S3260M4 PowerMetrics - " + err.Error())
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
		(*pow)["supplyTotalConsumed"].WithLabelValues(pm.Url, e.chassisSerialNumber).Set(watts)
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
			(*pow)["voltageStatus"].WithLabelValues(pv.Name, e.chassisSerialNumber).Set(state)
		}
	}

	for _, ps := range pm.PowerSupplies {
		if ps.Status.State == "Enabled" {
			state = OK
		} else {
			state = DISABLED
		}
		(*pow)["supplyStatus"].WithLabelValues(ps.Name, e.chassisSerialNumber, ps.Manufacturer, ps.SerialNumber, ps.FirmwareVersion, ps.PowerSupplyType, ps.Model).Set(state)
	}

	return nil
}

// exportThermalMetrics collects the Cisco UCS S3260M4's thermal and fan metrics in json format and sets the prometheus gauges
func (e *Exporter) exportThermalMetrics(body []byte) error {

	var state float64
	var tm oem.ThermalMetrics
	var therm = (*e.deviceMetrics)["thermalMetrics"]
	err := json.Unmarshal(body, &tm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling S3260M4 ThermalMetrics - " + err.Error())
	}

	if tm.Status.State == "Enabled" {
		if tm.Status.Health == "OK" {
			state = OK
		} else {
			state = BAD
		}
		(*therm)["thermalSummary"].WithLabelValues(tm.Url, e.chassisSerialNumber).Set(state)
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

// exportMemoryMetrics collects the Cisco UCS S3260M4's memory metrics in json format and sets the prometheus gauges
func (e *Exporter) exportMemoryMetrics(body []byte) error {

	var state float64
	var mm oem.MemoryMetrics
	var mem = (*e.deviceMetrics)["memoryMetrics"]
	err := json.Unmarshal(body, &mm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling S3260M4 MemoryMetrics - " + err.Error())
	}

	if mm.Status != "" {
		var memCap string
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
		(*mem)["memoryStatus"].WithLabelValues(mm.Name, e.chassisSerialNumber, memCap, mm.Manufacturer, strings.TrimRight(mm.PartNumber, " "), mm.SerialNumber).Set(state)
	}

	return nil
}

// exportProcessorMetrics collects the Cisco UCS S3260M4's processor metrics in json format and sets the prometheus gauges
func (e *Exporter) exportProcessorMetrics(body []byte) error {

	var state float64
	var totThreads string
	var pm oem.ProcessorMetrics
	var proc = (*e.deviceMetrics)["processorMetrics"]
	err := json.Unmarshal(body, &pm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling S3260M4 ProcessorMetrics - " + err.Error())
	}

	if pm.Status.State == "Enabled" {
		switch pm.TotalThreads.(type) {
		case string:
			totThreads = pm.TotalThreads.(string)
		case float64:
			totThreads = strconv.Itoa(int(pm.TotalThreads.(float64)))
		case int:
			totThreads = strconv.Itoa(pm.TotalThreads.(int))
		}
		if pm.Status.Health == "OK" {
			state = OK
		} else {
			state = BAD
		}
	} else {
		state = DISABLED
	}

	(*proc)["processorStatus"].WithLabelValues(pm.Name, e.chassisSerialNumber, pm.Description, totThreads).Set(state)

	return nil
}

// exportDriveMetrics collects the Cisco UCS S3260M4's drive metrics in json format and sets the prometheus gauges
func (e *Exporter) exportDriveMetrics(body []byte) error {

	var state float64
	var scm oem.StorageControllerMetrics
	var dlDrive = (*e.deviceMetrics)["driveMetrics"]
	err := json.Unmarshal(body, &scm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling S3260M4 DriveMetrics - " + err.Error())
	}
	// Check logical drive is enabled then check status and convert string to numeric values
	for _, sc := range scm.StorageController.StorageController {
		if sc.Status.State == "Enabled" {
			if sc.Status.Health == "OK" {
				state = OK
			} else {
				state = BAD
			}
		} else {
			state = DISABLED
		}

		(*dlDrive)["storageControllerStatus"].WithLabelValues(sc.Name, e.chassisSerialNumber, sc.FirmwareVersion, sc.Manufacturer, sc.Model).Set(state)
	}

	return nil
}

func getManagerEndpoint(url, host string, client *retryablehttp.Client) (oem.Chassis, error) {
	var chas oem.Chassis
	req := common.BuildRequest(url, host)

	resp, err := client.Do(req)
	if err != nil {
		return chas, err
	}
	defer resp.Body.Close()
	if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
		if resp.StatusCode == http.StatusUnauthorized {
			return chas, common.ErrInvalidCredential
		} else {
			return chas, fmt.Errorf("HTTP status %d", resp.StatusCode)
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return chas, fmt.Errorf("Error reading Response Body - " + err.Error())
	}

	err = json.Unmarshal(body, &chas)
	if err != nil {
		return chas, fmt.Errorf("Error Unmarshalling S3260M4 Chassis struct - " + err.Error())
	}

	return chas, nil
}

func getChassisSerialNumber(url, host string, client *retryablehttp.Client) (string, error) {
	var chassSN oem.ChassisSerialNumber
	req := common.BuildRequest(url, host)

	resp, err := client.Do(req)
	if err != nil {
		return chassSN.SerialNumber, err
	}
	defer resp.Body.Close()
	if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
		return chassSN.SerialNumber, fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return chassSN.SerialNumber, fmt.Errorf("Error reading Response Body - " + err.Error())
	}

	err = json.Unmarshal(body, &chassSN)
	if err != nil {
		return chassSN.SerialNumber, fmt.Errorf("Error Unmarshalling S3260M4 Chassis struct - " + err.Error())
	}

	return chassSN.SerialNumber, nil
}

func getChassisEndpoint(url, host string, client *retryablehttp.Client) (oem.Collection, error) {
	var chas oem.Collection
	req := common.BuildRequest(url, host)

	resp, err := client.Do(req)
	if err != nil {
		return chas, err
	}
	defer resp.Body.Close()
	if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
		return chas, fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return chas, fmt.Errorf("Error reading Response Body - " + err.Error())
	}

	err = json.Unmarshal(body, &chas)
	if err != nil {
		return chas, fmt.Errorf("Error Unmarshalling S3260M4 Chassis struct - " + err.Error())
	}

	return chas, nil
}

func getBIOSVersion(url, host string, client *retryablehttp.Client) (string, error) {
	var biosVer oem.ServerManager
	req := common.BuildRequest(url, host)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
		return "", fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Error reading Response Body - " + err.Error())
	}

	err = json.Unmarshal(body, &biosVer)
	if err != nil {
		return "", fmt.Errorf("Error Unmarshalling S3260M4 ServerManager struct - " + err.Error())
	}

	return biosVer.BiosVersion, nil
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
			for retryCount < 3 && resp.StatusCode == http.StatusNotFound {
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
		return dimms, fmt.Errorf("Error Unmarshalling S3260M4 Memory Collection struct - " + err.Error())
	}

	return dimms, nil
}

func getRaidEndpoint(url, host string, client *retryablehttp.Client) (oem.Collection, error) {
	var rcontrollers oem.Collection
	var resp *http.Response
	var err error
	retryCount := 0
	req := common.BuildRequest(url, host)

	resp, err = common.DoRequest(client, req)
	if err != nil {
		return rcontrollers, err
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
				return rcontrollers, err
			} else if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
				return rcontrollers, fmt.Errorf("HTTP status %d", resp.StatusCode)
			}
		} else {
			return rcontrollers, fmt.Errorf("HTTP status %d", resp.StatusCode)
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return rcontrollers, fmt.Errorf("Error reading Response Body - " + err.Error())
	}

	err = json.Unmarshal(body, &rcontrollers)
	if err != nil {
		return rcontrollers, fmt.Errorf("Error Unmarshalling S3260M5 Chassis struct - " + err.Error())
	}

	return rcontrollers, nil
}
