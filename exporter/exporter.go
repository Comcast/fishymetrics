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
	"github.com/comcast/fishymetrics/oem"
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
	// UNKNOWN_DRIVE is placeholder for unknown drive types
	UNKNOWN_DRIVE = "UnknownDriveMetrics"
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
	ctx         context.Context
	mutex       sync.RWMutex
	pool        *pool.Pool
	client      *retryablehttp.Client
	host        string
	url         string
	credProfile string
	biosVersion string

	ChassisSerialNumber string
	DeviceMetrics       *map[string]*metrics
	Model               string
}

type SystemEndpoints struct {
	storageController []string
	drives            []string
	systems           []string
	power             []string
	thermal           []string
}

type DriveEndpoints struct {
	logicalDriveURLs  []string
	physicalDriveURLs []string
}

type Excludes map[string]interface{}

type Plugin interface {
	Apply(*Exporter) error
}

// NewExporter returns an initialized Exporter for a redfish API capable device.
func NewExporter(ctx context.Context, target, uri, profile, model string, excludes Excludes, plugins ...Plugin) (*Exporter, error) {
	var fqdn *url.URL
	var tasks []*pool.Task
	var exp = Exporter{
		ctx:           ctx,
		credProfile:   profile,
		DeviceMetrics: NewDeviceMetrics(),
		Model:         model,
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

	exp.client = retryClient

	// Check that the target passed in has http:// or https:// prefixed
	fqdn, err := url.ParseRequestURI(target)
	if err != nil {
		fqdn = &url.URL{
			Scheme: config.GetConfig().BMCScheme,
			Host:   target,
		}
	}
	exp.host = fqdn.Hostname()
	exp.url = fqdn.String()

	// check if host is on the ignored list, if so we immediately return
	if _, ok := common.IgnoredDevices[exp.host]; ok {
		var upMetric = (*exp.DeviceMetrics)["up"]
		(*upMetric)["up"].WithLabelValues().Set(float64(2))
		return &exp, nil
	}

	chassisEndpoints, err := getMemberUrls(exp.url+uri+"/Chassis/", target, retryClient)
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
			var upMetric = (*exp.DeviceMetrics)["up"]
			(*upMetric)["up"].WithLabelValues().Set(float64(2))

			return &exp, nil
		}
		return nil, err
	}

	log.Debug("chassis endpoints response", zap.Strings("chassis_endpoints", chassisEndpoints), zap.Any("trace_id", ctx.Value("traceID")))

	mgrEndpoints, err := getMemberUrls(exp.url+uri+"/Managers/", target, retryClient)
	if err != nil {
		log.Error("error when getting manager endpoint from "+model, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		return nil, err
	}

	log.Debug("manager endpoints response", zap.Strings("mgr_endpoints", mgrEndpoints), zap.Any("trace_id", ctx.Value("traceID")))

	var mgrEndpointFinal string
	if len(mgrEndpoints) > 1 {
		for _, member := range mgrEndpoints {
			if !strings.Contains(member, "CIMC") {
				mgrEndpointFinal = member
				break
			}
		}
	} else if len(mgrEndpoints) > 0 {
		mgrEndpointFinal = mgrEndpoints[0]
	} else {
		return nil, errors.New("no manager endpoint found")
	}

	log.Debug("mgr endpoint final decision", zap.String("mgr_endpoint_final", mgrEndpointFinal), zap.Any("trace_id", ctx.Value("traceID")))

	// prepend the base url with the chassis url
	var chasUrlsFinal []string
	for _, chasUrl := range chassisEndpoints {
		chasUrlsFinal = append(chasUrlsFinal, exp.url+chasUrl)
	}

	log.Debug("chassis urls final", zap.Strings("chassis_urls_final", chasUrlsFinal), zap.Any("trace_id", ctx.Value("traceID")))

	// chassis endpoint to use for obtaining url endpoints for storage controller, NVMe drive metrics as well as the starting
	// point for the systems and manager endpoints
	sysEndpoints, err := getSystemEndpoints(chasUrlsFinal, target, retryClient, excludes)
	if err != nil {
		log.Error("error when getting chassis endpoints from "+model, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		return nil, err
	}

	log.Debug("systems endpoints response", zap.Strings("systems_endpoints", sysEndpoints.systems),
		zap.Strings("storage_ctrl_endpoints", sysEndpoints.storageController),
		zap.Strings("drives_endpoints", sysEndpoints.drives),
		zap.Strings("power_endpoints", sysEndpoints.power),
		zap.Strings("thermal_endpoints", sysEndpoints.thermal),
		zap.Any("trace_id", ctx.Value("traceID")))

	// check /redfish/v1/Systems/XXXXX/ exists so we don't panic
	var sysResp oem.System
	var dimms, processors oem.Collection
	if len(sysEndpoints.systems) > 0 {
		// call /redfish/v1/Systems/XXXXX/ for BIOS, Serial number
		sysResp, err = getSystemsMetadata(exp.url+sysEndpoints.systems[0], target, retryClient)
		if err != nil {
			log.Error("error when getting BIOS version from "+model, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
			return nil, err
		}
		exp.biosVersion = sysResp.BiosVersion
		exp.ChassisSerialNumber = sysResp.SerialNumber

		// call /redfish/v1/Systems/XXXXX/ for memory summary and smart storage batteries
		// TODO: do not assume 1 systems endpoint
		tasks = append(tasks,
			pool.NewTask(common.Fetch(exp.url+sysEndpoints.systems[0], target, profile, retryClient),
				exp.url+sysEndpoints.systems[0],
				handle(&exp, MEMORY_SUMMARY, STORAGEBATTERY)))

		// DIMM endpoints array
		dimms, err = getDIMMEndpoints(exp.url+sysEndpoints.systems[0]+"Memory/", target, retryClient)
		if err != nil {
			log.Error("error when getting DIMM endpoints from "+model, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
			return nil, err
		}

		// CPU processor metrics
		processors, err = getProcessorEndpoints(exp.url+sysEndpoints.systems[0]+"Processors/", target, retryClient)
		if err != nil {
			log.Error("error when getting Processors endpoints from "+model, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
			return nil, err
		}
	} else {
		return nil, errors.New("no systems endpoint found")
	}

	// check for SmartStorage endpoint from either Hp or Hpe
	var ss string
	if sysResp.Oem.Hpe.Links.SmartStorage.URL != "" {
		ss = appendSlash(sysResp.Oem.Hpe.Links.SmartStorage.URL) + "ArrayControllers/"
	} else if sysResp.Oem.Hp.Links.SmartStorage.URL != "" {
		ss = appendSlash(sysResp.Oem.Hp.Links.SmartStorage.URL) + "ArrayControllers/"
	} else if sysResp.Oem.Hpe.LinksLower.SmartStorage.URL != "" {
		ss = appendSlash(sysResp.Oem.Hpe.LinksLower.SmartStorage.URL) + "ArrayControllers/"
	} else if sysResp.Oem.Hp.LinksLower.SmartStorage.URL != "" {
		ss = appendSlash(sysResp.Oem.Hp.LinksLower.SmartStorage.URL) + "ArrayControllers/"
	}

	// skip if SmartStorage URL is not present
	var driveEndpointsResp DriveEndpoints
	if ss != "" {
		driveEndpointsResp, err = getAllDriveEndpoints(ctx, exp.url, exp.url+ss, target, retryClient)
		if err != nil {
			log.Error("error when getting drive endpoints from "+model, zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
			return nil, err
		}
	}

	log.Debug("drive endpoints response", zap.Strings("logical_drive_endpoints", driveEndpointsResp.logicalDriveURLs),
		zap.Strings("physical_drive_endpoints", driveEndpointsResp.physicalDriveURLs),
		zap.Any("trace_id", ctx.Value("traceID")))

	// Loop through logicalDriveURLs, physicalDriveURLs, and nvmeDriveURLs and append each URL to the tasks pool
	for _, url := range driveEndpointsResp.logicalDriveURLs {
		tasks = append(tasks, pool.NewTask(common.Fetch(exp.url+url, target, profile, retryClient), exp.url+url, handle(&exp, LOGICALDRIVE)))
	}

	for _, url := range driveEndpointsResp.physicalDriveURLs {
		tasks = append(tasks, pool.NewTask(common.Fetch(exp.url+url, target, profile, retryClient), exp.url+url, handle(&exp, DISKDRIVE)))
	}

	// drives from this list could either be NVMe or physical SAS/SATA
	for _, url := range sysEndpoints.drives {
		tasks = append(tasks, pool.NewTask(common.Fetch(exp.url+url, target, profile, retryClient), exp.url+url, handle(&exp, UNKNOWN_DRIVE)))
	}

	// storage controller
	for _, url := range sysEndpoints.storageController {
		tasks = append(tasks, pool.NewTask(common.Fetch(exp.url+url, target, profile, retryClient), exp.url+url, handle(&exp, STORAGE_CONTROLLER)))
	}

	// power
	for _, url := range sysEndpoints.power {
		tasks = append(tasks, pool.NewTask(common.Fetch(exp.url+url, target, profile, retryClient), exp.url+url, handle(&exp, POWER)))
	}

	// thermal
	for _, url := range sysEndpoints.thermal {
		tasks = append(tasks, pool.NewTask(common.Fetch(exp.url+url, target, profile, retryClient), exp.url+url, handle(&exp, THERMAL)))
	}

	// DIMMs
	for _, dimm := range dimms.Members {
		tasks = append(tasks,
			pool.NewTask(common.Fetch(exp.url+dimm.URL, target, profile, retryClient), exp.url+dimm.URL, handle(&exp, MEMORY)))
	}

	// call /redfish/v1/Managers/XXX/ for firmware version and ilo self test metrics
	tasks = append(tasks,
		pool.NewTask(common.Fetch(exp.url+mgrEndpointFinal, target, profile, retryClient),
			exp.url+mgrEndpointFinal,
			handle(&exp, FIRMWARE, ILOSELFTEST)))

	for _, processor := range processors.Members {
		tasks = append(tasks,
			pool.NewTask(common.Fetch(exp.url+processor.URL, target, profile, retryClient), exp.url+processor.URL, handle(&exp, PROCESSOR)))
	}

	exp.pool = pool.NewPool(tasks, 1)

	// check for any plugins, this feature allows one to collect any remaining component data not present inside
	// the redfish API.
	// Please see docs/plugins.md for more information.
	for _, plugin := range plugins {
		err := plugin.Apply(&exp)
		if err != nil {
			return &exp, err
		}
	}

	return &exp, nil
}

// Describe describes all the metrics ever exported by the fishymetrics exporter. It
// implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range *e.DeviceMetrics {
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
		var upMetric = (*e.DeviceMetrics)["up"]
		(*upMetric)["up"].WithLabelValues().Set(float64(2))
	}

	e.collectMetrics(ch)
}

func (e *Exporter) resetMetrics() {
	for _, m := range *e.DeviceMetrics {
		for _, n := range *m {
			n.Reset()
		}
	}
}

func (e *Exporter) collectMetrics(metrics chan<- prometheus.Metric) {
	for _, m := range *e.DeviceMetrics {
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
					Module:            e.Model,
					CredentialProfile: e.credProfile,
				}
				log.Info("added host "+e.host+" to ignored list", zap.Any("trace_id", e.ctx.Value("traceID")))
				deviceState = 2
			} else {
				deviceState = 0
			}
			var upMetric = (*e.DeviceMetrics)["up"]
			(*upMetric)["up"].WithLabelValues().Set(float64(deviceState))
			log.Error("error calling redfish api from "+e.Model, zap.Error(task.Err), zap.String("url", task.URL), zap.Any("trace_id", e.ctx.Value("traceID")))
			return
		}

		for _, handler := range task.MetricHandlers {
			err = handler(task.Body)
		}

		if err != nil {
			log.Error("error exporting metrics - from "+e.Model, zap.Error(err), zap.String("url", task.URL), zap.Any("trace_id", e.ctx.Value("traceID")))
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

	var upMetric = (*e.DeviceMetrics)["up"]
	(*upMetric)["up"].WithLabelValues().Set(float64(state))

}

func (e *Exporter) GetContext() context.Context {
	return e.ctx
}

func (e *Exporter) GetHost() string {
	return e.host
}

func (e *Exporter) GetUrl() string {
	return e.url
}

func (e *Exporter) GetPool() *pool.Pool {
	return e.pool
}

func (e *Exporter) GetClient() *retryablehttp.Client {
	return e.client
}
