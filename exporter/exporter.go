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

package exporter

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/comcast/fishymetrics/common"
	"github.com/comcast/fishymetrics/config"
	"github.com/comcast/fishymetrics/pool"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const (
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
	STORAGEBATTERY = "StorBatteryMetrics"
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
	model               string
	credProfile         string
	biosVersion         string
	chassisSerialNumber string
	deviceMetrics       *map[string]*metrics
}

// NewExporter returns an initialized Exporter for a redfish API capable device.
func NewExporter(ctx context.Context, target, uri, profile, model string) (*Exporter, error) {
	var fqdn *url.URL
	var tasks []*pool.Task
	var exp = Exporter{
		ctx:           ctx,
		credProfile:   profile,
		deviceMetrics: NewDeviceMetrics(),
	}
	var urls []string

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

	urls = append(urls, fqdn.String()+uri+"/Managers/1/") //* */
	// chassis system endpoint to use for memory, processor, bios version scrapes
	sysEndpoint, err := getChassisEndpoint(fqdn.String()+uri+"/Managers/1/", target, retryClient)
	if err != nil {
		log.Error("error when getting chassis endpoint from "+model, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		if errors.Is(err, common.ErrInvalidCredential) {
			common.IgnoredDevices[exp.host] = common.IgnoredDevice{
				Name:              exp.host,
				Endpoint:          "https://" + exp.host + "/redfish/v1/Chassis/",
				Module:            model,
				CredentialProfile: exp.credProfile,
			}
			log.Info("added host "+exp.host+" to ignored list", zap.Any("trace_id", exp.ctx.Value("traceID")))
			var upMetric = (*exp.deviceMetrics)["up"]
			(*upMetric)["up"].WithLabelValues().Set(float64(2))

			return &exp, nil
		}
		return nil, err
	}

	urls = append(urls, fqdn.String()+sysEndpoint) //* */
	resp, err := getSystemsMetadata(fqdn.String()+sysEndpoint, target, retryClient)
	if err != nil {
		log.Error("error when getting BIOS version from "+model, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		return nil, err
	}
	exp.biosVersion = resp.BiosVersion
	exp.chassisSerialNumber = resp.SerialNumber

	// vars for drive parsing
	var (
		initialURL        = "/Systems/1/SmartStorage/ArrayControllers/"
		url               = initialURL
		chassisUrl        = "/Chassis/1/"
		logicalDriveURLs  []string
		physicalDriveURLs []string
		nvmeDriveURLs     []string
	)

	urls = append(urls, fqdn.String()+uri+url) //* */
	// PARSING DRIVE ENDPOINTS
	// Get initial JSON return of /redfish/v1/Systems/1/SmartStorage/ArrayControllers/ set to output
	driveResp, err := getDriveEndpoint(fqdn.String()+uri+url, target, retryClient)
	if err != nil {
		log.Error("api call "+fqdn.String()+uri+url+" failed - ", zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		if errors.Is(err, common.ErrInvalidCredential) {
			common.IgnoredDevices[exp.host] = common.IgnoredDevice{
				Name:              exp.host,
				Endpoint:          "https://" + exp.host + "/redfish/v1/Chassis/",
				Module:            model,
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

	urls = append(urls, fqdn.String()+uri+chassisUrl) //* */
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

	urls = append(urls, fqdn.String()+sysEndpoint+"Memory/") //* */
	// DIMM endpoints array
	dimms, err := getDIMMEndpoints(fqdn.String()+sysEndpoint+"Memory/", target, retryClient)
	if err != nil {
		log.Error("error when getting DIMM endpoints from "+model, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		return nil, err
	}

	urls = append(urls, fqdn.String()+uri+"/Managers/1") //* */
	tasks = append(tasks,
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Managers/1", FIRMWARE, target, profile, retryClient)))

	urls = append(urls, fqdn.String()+uri+"/Chassis/1/Thermal/") //* */
	urls = append(urls, fqdn.String()+uri+"/Chassis/1/Power/")   //* */
	urls = append(urls, fqdn.String()+uri+"/Systems/1/")         //* */
	// Additional tasks for pool to perform
	tasks = append(tasks,
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Chassis/1/Thermal/", THERMAL, target, profile, retryClient)),
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Chassis/1/Power/", POWER, target, profile, retryClient)),
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Systems/1/", MEMORY_SUMMARY, target, profile, retryClient)))

	// DIMMs
	for _, dimm := range dimms.Members {
		urls = append(urls, fqdn.String()+dimm.URL) //* */
		tasks = append(tasks,
			pool.NewTask(common.Fetch(fqdn.String()+dimm.URL, MEMORY, target, profile, retryClient)))
	}

	urls = append(urls, fqdn.String()+uri+"/Systems/1/")  //* */
	urls = append(urls, fqdn.String()+uri+"/Managers/1/") //* */
	tasks = append(tasks,
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Systems/1/", STORAGEBATTERY, target, profile, retryClient)),
		pool.NewTask(common.Fetch(fqdn.String()+uri+"/Managers/1/", ILOSELFTEST, target, profile, retryClient)))

	urls = append(urls, fqdn.String()+uri+"/Systems/1/Processors/") //* */
	processors, err := getProcessorEndpoints(fqdn.String()+uri+"/Systems/1/Processors/", target, retryClient)
	if err != nil {
		log.Error("error when getting Processors endpoints from "+model, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		return nil, err
	}

	for _, processor := range processors.Members {
		urls = append(urls, fqdn.String()+processor.URL) //* */
		tasks = append(tasks,
			pool.NewTask(common.Fetch(fqdn.String()+processor.URL, PROCESSOR, target, profile, retryClient)))
	}

	exp.pool = pool.NewPool(tasks, 1)

	for _, url := range urls {
		fmt.Printf("%v\n", url)
	}

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
					Endpoint:          "https://" + e.host + "/redfish/v1/Chassis/",
					Module:            e.model,
					CredentialProfile: e.credProfile,
				}
				log.Info("added host "+e.host+" to ignored list", zap.Any("trace_id", e.ctx.Value("traceID")))
				deviceState = 2
			} else {
				deviceState = 0
			}
			var upMetric = (*e.deviceMetrics)["up"]
			(*upMetric)["up"].WithLabelValues().Set(float64(deviceState))
			log.Error("error from "+e.model, zap.Error(task.Err), zap.String("api", task.MetricType), zap.Any("trace_id", e.ctx.Value("traceID")))
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
			log.Error("error exporting metrics - from "+e.model, zap.Error(err), zap.String("api", task.MetricType), zap.Any("trace_id", e.ctx.Value("traceID")))
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
