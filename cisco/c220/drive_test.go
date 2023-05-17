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

package c220

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

const (
	driveHeader = `
		 # HELP c220_drive_status Current drive status 1 = OK, 0 = BAD, -1 = DISABLED, 2 = Max sesssions reached
		 # TYPE c220_drive_status gauge
	`

	scHeader = `
		 # HELP c220_storage_controller_status Current storage controller status 1 = OK, 0 = BAD, -1 = DISABLED, 2 = Max sesssions reached
		 # TYPE c220_storage_controller_status gauge
	`
)

func Test_C220_Drive_Metrics(t *testing.T) {

	var exporter prometheus.Collector

	assert := assert.New(t)

	HealthyStorageControllerMetricsResponse, _ := json.Marshal(struct {
		Name              string              `json:"Name"`
		Drives            []Drive             `json:"Drives"`
		StorageController []StorageController `json:"StorageControllers"`
	}{
		Name: "MRAID",
		Drives: []Drive{
			{
				Url: "/redfish/v1/Systems/WZP1111111/Storage/MRAID/Drives/PD-1",
			},
		},
		StorageController: []StorageController{
			{
				MemberId:        "RAID",
				Model:           "Cisco 12G Modular Raid Controller with 2GB cache (max 16 drives)",
				Name:            "Cisco 12G Modular Raid Controller with 2GB cache (max 16 drives)",
				FirmwareVersion: "555555",
				Status: Status{
					State:        "Enabled",
					Health:       "OK",
					HealthRollup: "OK",
				},
			},
		},
	})

	BadStorageControllerMetricsResponse, _ := json.Marshal(struct {
		Name              string              `json:"Name"`
		Drives            []Drive             `json:"Drives"`
		StorageController []StorageController `json:"StorageControllers"`
	}{
		Name:   "MRAID",
		Drives: []Drive{},
		StorageController: []StorageController{
			{
				MemberId:        "RAID",
				Model:           "Cisco 12G Modular Raid Controller with 2GB cache (max 16 drives)",
				Name:            "Cisco 12G Modular Raid Controller with 2GB cache (max 16 drives)",
				FirmwareVersion: "555555",
				Status: Status{
					State:        "Enabled",
					Health:       "BAD",
					HealthRollup: "BAD",
				},
			},
		},
	})

	// MissingStorageControllerMetricsResponse, _ := json.Marshal(Error{
	// 	Error: ErrorBody{
	// 		Code:    "Base.1.4.GeneralError",
	// 		Message: "See ExtendedInfo for more information.",
	// 		ExtendedInfo: []ExtendedInfo{
	// 			{
	// 				OdataType:  "Message.v1_0_6.Message",
	// 				MessageID:  "Base.1.4.ResourceNotFound",
	// 				Message:    "The resource 'MRAID' does not exist.",
	// 				MessageArg: []string{"MRAID"},
	// 				Severity:   "Critical",
	// 			},
	// 		},
	// 	},
	// })

	metrx := NewDeviceMetrics()

	exporter = &Exporter{
		ctx:                 context.Background(),
		host:                "fishymetrics.com",
		biosVersion:         "C220M5.4.0.4i.0.zzzzzzzzz",
		chassisSerialNumber: "SN78901",
		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "up",
			Help: "Was the last scrape of chassis monitor successful.",
		}),
		deviceMetrics: metrx,
	}

	prometheus.MustRegister(exporter)

	tests := []struct {
		name         string
		response     []byte
		metricHeader string
		expected     string
	}{
		{
			name:         "Healthy Storage Controller Metrics",
			response:     HealthyStorageControllerMetricsResponse,
			metricHeader: scHeader,
			expected: `
				c220_storage_controller_status{chassisSerialNumber="SN78901",firmwareVersion="555555",memberId="RAID",model="Cisco 12G Modular Raid Controller with 2GB cache (max 16 drives)",name="Cisco 12G Modular Raid Controller with 2GB cache (max 16 drives)"} 1
			`,
		},
		{
			name:         "Bad Storage Controller Metrics",
			response:     BadStorageControllerMetricsResponse,
			metricHeader: scHeader,
			expected: `
			c220_storage_controller_status{chassisSerialNumber="SN78901",firmwareVersion="555555",memberId="RAID",model="Cisco 12G Modular Raid Controller with 2GB cache (max 16 drives)",name="Cisco 12G Modular Raid Controller with 2GB cache (max 16 drives)"} 0
			`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := exporter.(*Exporter).exportStorageControllerMetrics(test.response)
			if err != nil {
				t.Error(err)
			}

			drv := (*exporter.(*Exporter).deviceMetrics)["driveMetrics"]
			drvMetrics := (*drv)["storageControllerStatus"]

			assert.Empty(testutil.CollectAndCompare(drvMetrics, strings.NewReader(test.metricHeader+test.expected), "c220_storage_controller_status"))

			drvMetrics.Reset()

		})
	}

	prometheus.Unregister(exporter)

}
