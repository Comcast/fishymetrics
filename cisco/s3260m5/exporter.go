/*
 * Copyright 2023 Comcast Cable Communications Management, LLC
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

package s3260m5

import (
	"context"
	"crypto/tls"
	"encoding/json"
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
	"github.com/comcast/fishymetrics/pool"
	"go.uber.org/zap"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/hashicorp/go-version"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// S3260M5 is a Cisco Hardware Device we scrape
	S3260M5 = "s3260m5"
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

	up            prometheus.Gauge
	deviceMetrics *map[string]*metrics
}

// NewExporter returns an initialized Exporter for Cisco UCS S3260M5 device.
func NewExporter(ctx context.Context, target, uri, profile string) (*Exporter, error) {
	var fqdn *url.URL
	var tasks []*pool.Task
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

	// chassis system endpoint to use for memory, processor, bios version scrapes
	mgrEndpoints, err := getManagerEndpoint(fqdn.String()+uri+"/Managers/BMC1", target, retryClient)
	if err != nil {
		log.Error("error when getting managers endpoint from "+S3260M5, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		return nil, err
	}

	if len(mgrEndpoints.Links.ServerManager) > 0 {
		mgr = mgrEndpoints.Links.ServerManager[0].URL
	}

	// BMC Firmware major.minor
	bmcFwTrim, err := version.NewVersion(mgrEndpoints.FirmwareVersion[:strings.Index(mgrEndpoints.FirmwareVersion, "(")])
	if err != nil {
		log.Error("error when trimming BMC FW version from "+S3260M5, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		return nil, err
	}

	// chassis BIOS version
	biosVer, err := getBIOSVersion(fqdn.String()+mgr, target, retryClient)
	if err != nil {
		log.Error("error when getting BIOS version from "+S3260M5, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		return nil, err
	}

	// DIMM endpoints array
	dimms, err := getDIMMEndpoints(fqdn.String()+mgr+"/Memory", target, retryClient)
	if err != nil {
		log.Error("error when getting DIMM endpoints from "+S3260M5, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		return nil, err
	}

	// chassis CMC/Server1/Server2 endpoints
	chass, err := getChassisEndpoint(fqdn.String()+uri+"/Chassis", target, retryClient)
	if err != nil {
		log.Error("error when getting chassis endpoint from "+S3260M5, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		return nil, err
	}

	// chassis serial number
	chassisSN, err := getChassisSerialNumber(fqdn.String()+chass.Members[0].URL, target, retryClient)
	if err != nil {
		log.Error("error when getting chassis serial number from "+S3260M5, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		return nil, err
	}

	serial := path.Base(mgr)

	tasks = append(tasks,
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Managers/CIMC", FIRMWARE, target, profile, retryClient)))

	for _, ch := range chass.Members {
		tasks = append(tasks,
			pool.NewTask(common.Fetch(fqdn.String()+ch.URL+"/Thermal", THERMAL, target, profile, retryClient)),
		)
		constraints, _ := version.NewConstraint(">= 4.2")
		if constraints.Check(bmcFwTrim) && !strings.Contains(ch.URL, "Server1") {
			tasks = append(tasks,
				pool.NewTask(common.Fetch(fqdn.String()+ch.URL+"/Power", POWER, target, profile, retryClient)),
			)
		} else if !constraints.Check(bmcFwTrim) {
			tasks = append(tasks,
				pool.NewTask(common.Fetch(fqdn.String()+ch.URL+"/Power", POWER, target, profile, retryClient)),
			)
		}
	}

	tasks = append(tasks,
		pool.NewTask(common.Fetch(fqdn.String()+mgr+"/Processors/CPU1", PROCESSOR, target, profile, retryClient)),
		pool.NewTask(common.Fetch(fqdn.String()+mgr+"/Processors/CPU2", PROCESSOR, target, profile, retryClient)),
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Systems/"+serial+"/Storage/SBMezz1", DRIVE, target, profile, retryClient)),
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Systems/"+serial+"/Storage/SBMezz2", DRIVE, target, profile, retryClient)))

	for _, dimm := range dimms.Members {
		tasks = append(tasks,
			pool.NewTask(common.Fetch(fqdn.String()+dimm.URL, MEMORY, target, profile, retryClient)))
	}

	p := pool.NewPool(tasks, 1)

	// Create new map[string]*metrics for each new Exporter
	metrx := NewDeviceMetrics()

	return &Exporter{
		ctx:                 ctx,
		pool:                p,
		host:                fqdn.Host,
		credProfile:         profile,
		biosVersion:         biosVer,
		chassisSerialNumber: chassisSN,
		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "up",
			Help: "Was the last scrape of chassis monitor successful.",
		}),
		deviceMetrics: metrx,
	}, nil
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
					Module:            S3260M5,
					CredentialProfile: e.credProfile,
				}
				log.Info("added host "+e.host+" to ignored list", zap.Any("trace_id", e.ctx.Value("traceID")))
				deviceState = 2
			} else {
				deviceState = 0
			}
			e.up.Set(float64(deviceState))
			log.Error("error from "+S3260M5, zap.Error(task.Err), zap.String("api", task.MetricType), zap.Any("trace_id", e.ctx.Value("traceID")))
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
			log.Error("error exporting metrics - from "+S3260M5, zap.Error(err), zap.String("api", task.MetricType), zap.Any("trace_id", e.ctx.Value("traceID")))
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

// exportFirmwareMetrics collects the Cisco UCS S3260M5's device metrics in json format and sets the prometheus gauges
func (e *Exporter) exportFirmwareMetrics(body []byte) error {
	var chas Chassis
	var dm = (*e.deviceMetrics)["deviceInfo"]
	err := json.Unmarshal(body, &chas)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling S3260M5 FirmwareMetrics - " + err.Error())
	}

	(*dm)["deviceInfo"].WithLabelValues(chas.Description, e.chassisSerialNumber, chas.FirmwareVersion, e.biosVersion, S3260M5).Set(1.0)

	return nil
}

// exportPowerMetrics collects the Cisco UCS S3260M5's power metrics in json format and sets the prometheus gauges
func (e *Exporter) exportPowerMetrics(body []byte) error {
	var state float64
	var watts float64
	var pm PowerMetrics
	var pow = (*e.deviceMetrics)["powerMetrics"]
	err := json.Unmarshal(body, &pm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling S3260M5 PowerMetrics - " + err.Error())
	}

	for _, pc := range pm.PowerControl {
		if pc.PowerConsumedWatts == 0 {
			// use the AverageConsumedWatts if PowerConsumedWatts is not present
			watts = float64(pc.PowerMetrics.AverageConsumedWatts)
		} else {
			watts = float64(pc.PowerConsumedWatts)
		}
		(*pow)["supplyTotalConsumed"].WithLabelValues(pm.Url, e.chassisSerialNumber).Set(watts)
	}

	for _, pv := range pm.Voltages {
		if pv.Status.State == "Enabled" {
			(*pow)["voltageOutput"].WithLabelValues(pv.Name, e.chassisSerialNumber).Set(pv.ReadingVolts)
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
			(*pow)["supplyOutput"].WithLabelValues(ps.Name, e.chassisSerialNumber, ps.Manufacturer, ps.SerialNumber, ps.FirmwareVersion, ps.PowerSupplyType, ps.Model).Set(float64(ps.LastPowerOutputWatts))
		} else {
			state = DISABLED
		}
		(*pow)["supplyStatus"].WithLabelValues(ps.Name, e.chassisSerialNumber, ps.Manufacturer, ps.SerialNumber, ps.FirmwareVersion, ps.PowerSupplyType, ps.Model).Set(state)
	}

	return nil
}

// exportThermalMetrics collects the Cisco UCS S3260M5's thermal and fan metrics in json format and sets the prometheus gauges
func (e *Exporter) exportThermalMetrics(body []byte) error {

	var state float64
	var tm ThermalMetrics
	var therm = (*e.deviceMetrics)["thermalMetrics"]
	err := json.Unmarshal(body, &tm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling S3260M5 ThermalMetrics - " + err.Error())
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
			(*therm)["fanSpeed"].WithLabelValues(fan.Name, e.chassisSerialNumber).Set(float64(fan.Reading))
			if fan.Status.Health == "OK" {
				state = OK
			} else {
				state = BAD
			}
			(*therm)["fanStatus"].WithLabelValues(fan.Name, e.chassisSerialNumber).Set(state)
		}
	}

	// Iterate through sensors
	for _, sensor := range tm.Temperatures {
		// Check sensor status and convert string to numeric values
		if sensor.Status.State == "Enabled" {
			(*therm)["sensorTemperature"].WithLabelValues(strings.TrimRight(sensor.Name, " "), e.chassisSerialNumber).Set(sensor.ReadingCelsius)
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

// exportMemoryMetrics collects the Cisco UCS S3260M5's memory metrics in json format and sets the prometheus gauges
func (e *Exporter) exportMemoryMetrics(body []byte) error {

	var state float64
	var mm MemoryMetrics
	var mem = (*e.deviceMetrics)["memoryMetrics"]
	err := json.Unmarshal(body, &mm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling S3260M5 MemoryMetrics - " + err.Error())
	}

	if mm.Status.State != "" {
		if mm.Status.State == "Absent" {
			return nil
		} else if mm.Status.State == "Enabled" && mm.Status.Health == "OK" {
			state = OK
		} else {
			state = BAD
		}
	} else {
		state = DISABLED
	}

	(*mem)["memoryStatus"].WithLabelValues(mm.Name, e.chassisSerialNumber, strconv.Itoa(mm.CapacityMiB), mm.Manufacturer, mm.PartNumber, mm.SerialNumber).Set(state)

	return nil
}

// exportProcessorMetrics collects the Cisco UCS S3260M5's processor metrics in json format and sets the prometheus gauges
func (e *Exporter) exportProcessorMetrics(body []byte) error {

	var state float64
	var pm ProcessorMetrics
	var proc = (*e.deviceMetrics)["processorMetrics"]
	err := json.Unmarshal(body, &pm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling S3260M5 ProcessorMetrics - " + err.Error())
	}

	if pm.Status.State == "Enabled" {
		if pm.Status.Health == "OK" {
			state = OK
		} else {
			state = BAD
		}
	} else {
		state = DISABLED
	}

	(*proc)["processorStatus"].WithLabelValues(pm.Name, e.chassisSerialNumber, pm.Description, strconv.Itoa(pm.TotalThreads)).Set(state)

	return nil
}

// exportDriveMetrics collects the Cisco UCS S3260M5's drive metrics in json format and sets the prometheus gauges
func (e *Exporter) exportDriveMetrics(body []byte) error {

	var state float64
	var scm StorageControllerMetrics
	var dlDrive = (*e.deviceMetrics)["driveMetrics"]
	err := json.Unmarshal(body, &scm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling S3260M5 DriveMetrics - " + err.Error())
	}
	// Check logical drive is enabled then check status and convert string to numeric values
	for _, sc := range scm.StorageControllers {
		if sc.Status.State == "Enabled" {
			if sc.Status.Health == "OK" {
				state = OK
			} else {
				state = BAD
			}
		} else {
			state = DISABLED
		}

		(*dlDrive)["storageControllerStatus"].WithLabelValues(scm.Name, e.chassisSerialNumber, sc.FirmwareVersion, sc.Manufacturer, sc.Name).Set(state)
	}

	return nil
}

func getManagerEndpoint(url, host string, client *retryablehttp.Client) (Chassis, error) {
	var chas Chassis
	var resp *http.Response
	var err error
	retryCount := 0
	req := common.BuildRequest(url, host)

	resp, err = common.DoRequest(client, req)
	if err != nil {
		return chas, err
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
				return chas, err
			} else if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
				return chas, fmt.Errorf("HTTP status %d", resp.StatusCode)
			}
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
		return chas, fmt.Errorf("Error Unmarshalling S3260M5 Chassis struct - " + err.Error())
	}

	return chas, nil
}

func getChassisSerialNumber(url, host string, client *retryablehttp.Client) (string, error) {
	var chassSN ChassisSerialNumber
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
		return chassSN.SerialNumber, fmt.Errorf("Error Unmarshalling S3260M5 Chassis struct - " + err.Error())
	}

	return chassSN.SerialNumber, nil
}

func getChassisEndpoint(url, host string, client *retryablehttp.Client) (Collection, error) {
	var chas Collection
	var resp *http.Response
	var err error
	retryCount := 0
	req := common.BuildRequest(url, host)

	resp, err = common.DoRequest(client, req)
	if err != nil {
		return chas, err
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
				return chas, err
			} else if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
				return chas, fmt.Errorf("HTTP status %d", resp.StatusCode)
			}
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
		return chas, fmt.Errorf("Error Unmarshalling S3260M5 Chassis struct - " + err.Error())
	}

	return chas, nil
}

func getBIOSVersion(url, host string, client *retryablehttp.Client) (string, error) {
	var biosVer ServerManager
	var resp *http.Response
	var err error
	retryCount := 0
	req := common.BuildRequest(url, host)

	resp, err = common.DoRequest(client, req)
	if err != nil {
		return "", err
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
				return "", err
			} else if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
				return "", fmt.Errorf("HTTP status %d", resp.StatusCode)
			}
		} else {
			return "", fmt.Errorf("HTTP status %d", resp.StatusCode)
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Error reading Response Body - " + err.Error())
	}

	err = json.Unmarshal(body, &biosVer)
	if err != nil {
		return "", fmt.Errorf("Error Unmarshalling S3260M5 ServerManager struct - " + err.Error())
	}

	return biosVer.BiosVersion, nil
}

func getDIMMEndpoints(url, host string, client *retryablehttp.Client) (Collection, error) {
	var dimms Collection
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
		return dimms, fmt.Errorf("Error Unmarshalling S3260M5 Memory Collection struct - " + err.Error())
	}

	return dimms, nil
}
