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
	// STORAGE_CONTROLLER represents the MRAID metric endpoints
	STORAGE_CONTROLLER = "StorageControllerMetrics"
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

type SystemEndpoints struct {
	storageController []string
	drives            []string
	systems           []string
	manager           string
}

type DriveEndpoints struct {
	logicalDriveURLs  []string
	physicalDriveURLs []string
}

type Excludes map[string]interface{}

type Plugin interface {
	apply(*Exporter) error
}

type pluginFunc func(*Exporter) error

func (f pluginFunc) apply(exp *Exporter) error {
	err := f(exp)
	if err != nil {
		return err
	}
	return nil
}

// NewExporter returns an initialized Exporter for a redfish API capable device.
func NewExporter(ctx context.Context, target, uri, profile, model string, excludes Excludes, plugins ...Plugin) (*Exporter, error) {
	var fqdn *url.URL
	var tasks []*pool.Task
	var exp = Exporter{
		ctx:           ctx,
		credProfile:   profile,
		deviceMetrics: NewDeviceMetrics(),
		model:         model,
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

	chassisEndpoints, err := getChassisUrls(fqdn.String()+uri+"/Chassis/", target, retryClient)
	if err != nil {
		log.Error("error when getting chassis url from "+model, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
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

	// thermal and power metrics are obtained from all chassis endpoints
	for _, chasUrl := range chassisEndpoints {
		tasks = append(tasks,
			pool.NewTask(common.Fetch(fqdn.String()+chasUrl+"Thermal/", target, profile, retryClient), handle(&exp, THERMAL)),
			pool.NewTask(common.Fetch(fqdn.String()+chasUrl+"Power/", target, profile, retryClient), handle(&exp, POWER)))
	}

	// ignore /redfish/v1/Chassis/CMC/ endpoint from here on out except for cisco c220
	// TODO: handle cases where there are multiple non CMC chassis endpoints
	var chasUrlFinal string
	if len(chassisEndpoints) > 1 {
		for _, member := range chassisEndpoints {
			if !strings.Contains(member, "/CMC/") {
				chasUrlFinal = member
				break
			}
		}
	} else {
		chasUrlFinal = chassisEndpoints[0]
	}

	// chassis endpoint to use for obtaining url endpoints for storage controller, NVMe drive metrics as well as the starting
	// point for the systems and manager endpoints
	sysEndpoints, err := getSystemEndpoints(fqdn.String()+chasUrlFinal, target, retryClient, excludes)
	if err != nil {
		log.Error("error when getting chassis endpoints from "+model, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		return nil, err
	}

	// call /redfish/v1/Systems/XXXXX/ for BIOS, Serial number
	sysResp, err := getSystemsMetadata(fqdn.String()+sysEndpoints.systems[0], target, retryClient)
	if err != nil {
		log.Error("error when getting BIOS version from "+model, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		return nil, err
	}
	exp.biosVersion = sysResp.BiosVersion
	exp.chassisSerialNumber = sysResp.SerialNumber

	// call /redfish/v1/Systems/XXXXX/ for memory summary and smart storage batteries
	// TODO: do not assume 1 systems endpoint
	tasks = append(tasks,
		pool.NewTask(common.Fetch(fqdn.String()+sysEndpoints.systems[0], target, profile, retryClient),
			handle(&exp, MEMORY_SUMMARY, STORAGEBATTERY)))

	// check /redfish/v1/Chassis/XXXXX/ for smart storage batteries incase it is not in the systems endpoint
	tasks = append(tasks, pool.NewTask(common.Fetch(fqdn.String()+chasUrlFinal, target, profile, retryClient), handle(&exp, STORAGEBATTERY)))

	// check for SmartStorage endpoint from either Hp or Hpe
	var ss string
	if sysResp.Oem.Hpe.Links.SmartStorage.URL != "" {
		ss = appendSlash(sysResp.Oem.Hpe.Links.SmartStorage.URL) + "ArrayControllers/"
	} else if sysResp.Oem.Hp.Links.SmartStorage.URL != "" {
		ss = appendSlash(sysResp.Oem.Hp.Links.SmartStorage.URL) + "ArrayControllers/"
	}

	// skip if SmartStorage URL is not present
	var driveEndpointsResp DriveEndpoints
	if ss != "" {
		driveEndpointsResp, err = getAllDriveEndpoints(ctx, fqdn.String(), fqdn.String()+ss, target, retryClient)
		if err != nil {
			log.Error("error when getting drive endpoints from "+model, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
			return nil, err
		}
	}

	// Loop through logicalDriveURLs, physicalDriveURLs, and nvmeDriveURLs and append each URL to the tasks pool
	for _, url := range driveEndpointsResp.logicalDriveURLs {
		tasks = append(tasks, pool.NewTask(common.Fetch(fqdn.String()+url, target, profile, retryClient), handle(&exp, LOGICALDRIVE)))
	}

	for _, url := range driveEndpointsResp.physicalDriveURLs {
		tasks = append(tasks, pool.NewTask(common.Fetch(fqdn.String()+url, target, profile, retryClient), handle(&exp, DISKDRIVE)))
	}

	for _, url := range sysEndpoints.drives {
		tasks = append(tasks, pool.NewTask(common.Fetch(fqdn.String()+url, target, profile, retryClient), handle(&exp, NVME)))
	}

	// storage controller
	for _, url := range sysEndpoints.storageController {
		tasks = append(tasks, pool.NewTask(common.Fetch(fqdn.String()+url, target, profile, retryClient), handle(&exp, STORAGE_CONTROLLER)))
	}

	// DIMM endpoints array
	dimms, err := getDIMMEndpoints(fqdn.String()+sysEndpoints.systems[0]+"Memory/", target, retryClient)
	if err != nil {
		log.Error("error when getting DIMM endpoints from "+model, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		return nil, err
	}

	// DIMMs
	for _, dimm := range dimms.Members {
		tasks = append(tasks,
			pool.NewTask(common.Fetch(fqdn.String()+dimm.URL, target, profile, retryClient), handle(&exp, MEMORY)))
	}

	// call /redfish/v1/Managers/XXX/ for firmware version and ilo self test metrics
	tasks = append(tasks,
		pool.NewTask(common.Fetch(fqdn.String()+sysEndpoints.manager, target, profile, retryClient),
			handle(&exp, FIRMWARE, ILOSELFTEST)))

	// CPU processor metrics
	processors, err := getProcessorEndpoints(fqdn.String()+sysEndpoints.systems[0]+"Processors/", target, retryClient)
	if err != nil {
		log.Error("error when getting Processors endpoints from "+model, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		return nil, err
	}

	for _, processor := range processors.Members {
		tasks = append(tasks,
			pool.NewTask(common.Fetch(fqdn.String()+processor.URL, target, profile, retryClient), handle(&exp, PROCESSOR)))
	}

	// check for any plugins, this feature allows one to collect any remaining component data not present inside the redfish API
	for _, plug := range plugins {
		err := plug.apply(&exp)
		if err != nil {
			return &exp, err
		}
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
			log.Error("error calling redfish api from "+e.model, zap.Error(task.Err), zap.Any("trace_id", e.ctx.Value("traceID")))
			return
		}

		for _, handler := range task.MetricHandlers {
			err = handler(task.Body)
		}

		if err != nil {
			log.Error("error exporting metrics - from "+e.model, zap.Error(err), zap.Any("trace_id", e.ctx.Value("traceID")))
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
