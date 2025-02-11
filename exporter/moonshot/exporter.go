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

package moonshot

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
	"github.com/comcast/fishymetrics/oem/moonshot"
	"github.com/comcast/fishymetrics/pool"
	"go.uber.org/zap"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// MOONSHOT is a HPE Hardware Device we scrape
	MOONSHOT = "Moonshot"
	// SWITCHA is the Moonshot Hardware Device we scrape
	SWITCHA = "SwitchA"
	// SWITCHB is the Moonshot Hardware Device we scrape
	SWITCHB = "SwitchB"
	// THERMAL represents the thermal metric endpoint
	THERMAL = "ThermalMetrics"
	// POWER represents the power metric endpoint
	POWER = "PowerMetrics"
	// SWITCH represents the Moonshot switch metric endpoint
	SWITCH = "SwitchMetrics"
	// OK is a string representation of the float 1.0 for device status
	OK = 1.0
	// BAD is a string representation of the float 0.0 for device status
	BAD = 0.0
)

var (
	log *zap.Logger
)

// Exporter collects chassis manager stats from the given URI and exports them using
// the prometheus metrics package.
type Exporter struct {
	ctx           context.Context
	mutex         sync.RWMutex
	pool          *pool.MoonshotPool
	host          string
	credProfile   string
	deviceMetrics *map[string]*metrics
}

// NewExporter returns an initialized Exporter for HPE Moonshot device.
func NewExporter(ctx context.Context, target, uri, profile string) (*Exporter, error) {
	var fqdn *url.URL
	var tasks []*pool.MoonshotTask
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
			InsecureSkipVerify: config.GetConfig().SSLVerify,
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

	tasks = append(tasks,
		pool.NewMoonshotTask(fetch(fqdn.String()+uri+"/ThermalMetrics", MOONSHOT, THERMAL, target, profile, retryClient)),
		pool.NewMoonshotTask(fetch(fqdn.String()+uri+"/PowerMetrics", MOONSHOT, POWER, target, profile, retryClient)),
		pool.NewMoonshotTask(fetch(fqdn.String()+uri+"/switches/sa", SWITCHA, SWITCH, target, profile, retryClient)),
		pool.NewMoonshotTask(fetch(fqdn.String()+uri+"/switches/sb", SWITCHB, SWITCH, target, profile, retryClient)),
		pool.NewMoonshotTask(fetch(fqdn.String()+uri+"/switches/sa/ThermalMetrics", SWITCHA, THERMAL, target, profile, retryClient)),
		pool.NewMoonshotTask(fetch(fqdn.String()+uri+"/switches/sb/ThermalMetrics", SWITCHB, THERMAL, target, profile, retryClient)),
		pool.NewMoonshotTask(fetch(fqdn.String()+uri+"/switches/sa/PowerMetrics", SWITCHA, POWER, target, profile, retryClient)),
		pool.NewMoonshotTask(fetch(fqdn.String()+uri+"/switches/sb/PowerMetrics", SWITCHB, POWER, target, profile, retryClient)))

	exp.pool = pool.NewMoonshotPool(tasks, 1)

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

func fetch(uri, device, metricType, host, profile string, client *retryablehttp.Client) func() ([]byte, string, string, error) {
	var resp *http.Response
	var credential *common.Credential
	var err error
	retryCount := 0

	return func() ([]byte, string, string, error) {
		req := common.BuildRequest(uri, host)
		resp, err = common.DoRequest(client, req)
		if err != nil {
			return nil, device, metricType, err
		}
		defer common.EmptyAndCloseBody(resp)
		if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
			if resp.StatusCode == http.StatusNotFound {
				for retryCount < 3 && resp.StatusCode == http.StatusNotFound {
					time.Sleep(client.RetryWaitMin)
					resp, err = common.DoRequest(client, req)
					if err != nil {
						return nil, device, metricType, err
					}
					defer common.EmptyAndCloseBody(resp)
					retryCount = retryCount + 1
				}
				if err != nil {
					return nil, device, metricType, err
				} else if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
					return nil, device, metricType, fmt.Errorf("HTTP status %d", resp.StatusCode)
				}
			} else if resp.StatusCode == http.StatusUnauthorized {
				if common.ChassisCreds.Vault != nil {
					// Credentials may have rotated, go to vault and get the latest
					credential, err = common.ChassisCreds.GetCredentials(context.Background(), profile, host)
					if err != nil {
						return nil, device, metricType, fmt.Errorf("issue retrieving credentials from vault using target: %s", host)
					}
					common.ChassisCreds.Set(host, credential)
				} else {
					return nil, device, metricType, fmt.Errorf("HTTP status %d", resp.StatusCode)
				}

				// build new request with updated credentials
				req = common.BuildRequest(uri, host)

				time.Sleep(client.RetryWaitMin)
				resp, err = common.DoRequest(client, req)
				defer common.EmptyAndCloseBody(resp)
				if err != nil {
					return nil, device, metricType, fmt.Errorf("HTTP status %d", resp.StatusCode)
				}
			} else {
				return nil, device, metricType, fmt.Errorf("HTTP status %d", resp.StatusCode)
			}
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, device, metricType, fmt.Errorf("Error reading Response Body - " + err.Error())
		}
		return body, device, metricType, nil
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
					Endpoint:          "https://" + e.host + "/rest/v1/chassis/1",
					Model:             MOONSHOT,
					CredentialProfile: e.credProfile,
				}
				log.Info("added host "+e.host+" to ignored list", zap.Any("trace_id", e.ctx.Value("traceID")))
				deviceState = 2
			} else {
				deviceState = 0
			}
			var upMetric = (*e.deviceMetrics)["up"]
			(*upMetric)["up"].WithLabelValues().Set(float64(deviceState))
			log.Error("error from "+task.Device, zap.Error(task.Err), zap.String("api", task.MetricType), zap.Any("trace_id", e.ctx.Value("traceID")))
			return
		}

		switch task.Device {
		case MOONSHOT:
			switch task.MetricType {
			// Unmarshal and populate Moonshot Thermal metrics
			case THERMAL:
				err = e.exportThermalMetrics(task.Body)
			// Unmarshal and populate Moonshot Power metrics
			case POWER:
				err = e.exportPowerMetrics(task.Body)
			}
			// Unmarshal and populate Switch metrics
		case SWITCHA:
			switch task.MetricType {
			case SWITCH:
				err = e.exportSwitchMetrics(task.Body)
			// Unmarshal and populate SwitchA Thermal metrics
			case THERMAL:
				err = e.exportSwitchThermalMetrics("switch-a", task.Body)
			// Unmarshal and populate SwitchA Power metrics
			case POWER:
				err = e.exportSwitchPowerMetrics("switch-a", task.Body)
			}
		case SWITCHB:
			switch task.MetricType {
			case SWITCH:
				err = e.exportSwitchMetrics(task.Body)
			// Unmarshal and populate SwitchB Thermal metrics
			case THERMAL:
				err = e.exportSwitchThermalMetrics("switch-b", task.Body)
			// Unmarshal and populate SwitchB Power metrics
			case POWER:
				err = e.exportSwitchPowerMetrics("switch-b", task.Body)
			}
		}

		if err != nil {
			scrapeChan <- 0
			log.Error("error exporting metrics - from "+task.Device, zap.Error(err), zap.String("api", task.MetricType), zap.Any("trace_id", e.ctx.Value("traceID")))
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

// exportPowerMetrics collects the HPE Moonshot's power metrics in json format and sets the prometheus gauges
func (e *Exporter) exportPowerMetrics(body []byte) error {

	var state float64
	var pm moonshot.PowerMetrics
	var msPower = (*e.deviceMetrics)["powerMetrics"]
	err := json.Unmarshal(body, &pm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling PowerMetrics - " + err.Error())
	}

	(*msPower)["supplyTotalConsumed"].WithLabelValues().Set(float64(pm.PowerConsumedWatts))
	(*msPower)["supplyTotalCapacity"].WithLabelValues().Set(float64(pm.PowerCapacityWatts))
	for _, ps := range pm.PowerSupplies {
		(*msPower)["supplyOutput"].WithLabelValues(ps.Name, ps.SparePartNumber).Set(float64(ps.LastPowerOutputWatts))
		if ps.Status.State == "OK" {
			state = OK
		} else {
			state = BAD
		}
		(*msPower)["supplyStatus"].WithLabelValues(ps.Name, ps.SparePartNumber).Set(state)
	}

	return nil
}

// exportThermalMetrics collects the HPE Moonshot's thermal and fan metrics in json format and sets the prometheus gauges
func (e *Exporter) exportThermalMetrics(body []byte) error {

	var state float64
	var tm moonshot.ThermalMetrics
	var msThermal = (*e.deviceMetrics)["thermalMetrics"]
	err := json.Unmarshal(body, &tm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling ThermalMetrics - " + err.Error())
	}

	// Iterate through fans
	for _, fan := range tm.Fans {
		(*msThermal)["fanSpeed"].WithLabelValues(fan.FanName).Set(float64(fan.CurrentReading))
		// Check fan status and convert string to numeric values
		if fan.Status.State == "OK" {
			state = OK
		} else {
			state = BAD
		}
		(*msThermal)["fanStatus"].WithLabelValues(fan.FanName).Set(state)
	}

	// Iterate through sensors
	for _, sensor := range tm.Temperatures {
		(*msThermal)["sensorTemperature"].WithLabelValues(strings.TrimRight(sensor.Name, " ")).Set(float64(sensor.CurrentReading))
		// Check sensor status and convert string to numeric values
		if sensor.Status.State == "OK" {
			state = OK
		} else {
			state = BAD
		}
		(*msThermal)["sensorStatus"].WithLabelValues(strings.TrimRight(sensor.Name, " ")).Set(state)
	}

	return nil
}

// exportSwitchMetrics collects the switches metrics in json format and sets the prometheus gauges
func (e *Exporter) exportSwitchMetrics(body []byte) error {
	var state float64
	var sm moonshot.Sw
	var msSw = (*e.deviceMetrics)["swMetrics"]
	err := json.Unmarshal(body, &sm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling Sw - " + err.Error())
	}

	if sm.Status.State == "OK" {
		state = OK
	} else {
		state = BAD
	}
	(*msSw)["moonshotSwitchStatus"].WithLabelValues(sm.Name, sm.SerialNumber).Set(state)

	return nil
}

// exportSwitchThermalMetrics collects the switches thermal metrics in json format and sets the prometheus gauges
func (e *Exporter) exportSwitchThermalMetrics(namePrefix string, body []byte) error {

	var state float64
	var tm moonshot.ThermalMetrics
	var msSwThermal = (*e.deviceMetrics)["swThermalMetrics"]
	err := json.Unmarshal(body, &tm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling ThermalMetrics - " + err.Error())
	}

	// Iterate through sensors
	for _, sensor := range tm.Temperatures {
		(*msSwThermal)["moonshotSwitchSensorTemperature"].WithLabelValues(strings.TrimRight(namePrefix+"-"+tm.Name, " ")).Set(float64(sensor.CurrentReading))
		// Check sensor status and convert string to numeric values
		if sensor.Status.State == "OK" {
			state = OK
		} else {
			state = BAD
		}
		(*msSwThermal)["moonshotSwitchSensorStatus"].WithLabelValues(namePrefix + "-" + tm.Name).Set(state)
	}

	return nil
}

// exportSwitchPowerMetrics collects the switches power metrics in json format and sets the prometheus gauges
func (e *Exporter) exportSwitchPowerMetrics(namePrefix string, body []byte) error {

	var spm moonshot.SwPowerMetrics
	var msSwPower = (*e.deviceMetrics)["swPowerMetrics"]
	err := json.Unmarshal(body, &spm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling SwPowerMetrics - " + err.Error())
	}

	(*msSwPower)["moonshotSwitchSupplyOutput"].WithLabelValues(namePrefix + "-" + spm.Name).Set(float64(spm.Oem.Hp.InstantWattage))

	return nil
}
