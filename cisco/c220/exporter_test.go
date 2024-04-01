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

package c220

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

const (
	up2Expected = `
		 # HELP up was the last scrape of fishymetrics successful.
		 # TYPE up gauge
		 up 2
	`
	HealthyStorageControllerMetricsExpected = `
         # HELP c220_storage_controller_status Current storage controller status 1 = OK, 0 = BAD, -1 = DISABLED, 2 = Max sesssions reached
         # TYPE c220_storage_controller_status gauge
         c220_storage_controller_status{chassisSerialNumber="SN98765",firmwareVersion="555555",memberId="RAID",model="Cisco 12G Modular Raid Controller with 2GB cache (max 16 drives)",name="Cisco 12G Modular Raid Controller with 2GB cache (max 16 drives)"} 1
	`
	BadStorageControllerMetricsExpected = `
	     # HELP c220_storage_controller_status Current storage controller status 1 = OK, 0 = BAD, -1 = DISABLED, 2 = Max sesssions reached
         # TYPE c220_storage_controller_status gauge
         c220_storage_controller_status{chassisSerialNumber="SN98765",firmwareVersion="555555",memberId="RAID",model="Cisco 12G Modular Raid Controller with 2GB cache (max 16 drives)",name="Cisco 12G Modular Raid Controller with 2GB cache (max 16 drives)"} 0
	`
	HealthyFW4_0_4h_MemoryMetricsExpected = `
		 # HELP c220_memory_dimm_status Current dimm status 1 = OK, 0 = BAD
		 # TYPE c220_memory_dimm_status gauge
		 c220_memory_dimm_status{capacityMiB="32768",chassisSerialNumber="SN98765",manufacturer="0x2C00",name="DIMM_A1",partNumber="36ASF4G72PZ-2G6D1",serialNumber="SN123456"} 1
	`
	BadFW4_0_4h_MemoryMetricsExpected = `
		 # HELP c220_memory_dimm_status Current dimm status 1 = OK, 0 = BAD
 		 # TYPE c220_memory_dimm_status gauge
		 c220_memory_dimm_status{capacityMiB="32768",chassisSerialNumber="SN98765",manufacturer="0x2C00",name="DIMM_A1",partNumber="36ASF4G72PZ-2G6D1",serialNumber="SN123456"} 0
	`
	HealthyFW4_1_2a_MemoryMetricsExpected = `
	     # HELP c220_memory_dimm_status Current dimm status 1 = OK, 0 = BAD
 		 # TYPE c220_memory_dimm_status gauge
		 c220_memory_dimm_status{capacityMiB="32768",chassisSerialNumber="SN98765",manufacturer="0xAD00",name="DIMM_A1",partNumber="HMA84GR7DJR4N-WM",serialNumber="SN123456"} 1
	`
	BadFW4_1_2a_MemoryMetricsExpected = `
	     # HELP c220_memory_dimm_status Current dimm status 1 = OK, 0 = BAD
 		 # TYPE c220_memory_dimm_status gauge
		 c220_memory_dimm_status{capacityMiB="32768",chassisSerialNumber="SN98765",manufacturer="0xAD00",name="DIMM_A1",partNumber="HMA84GR7DJR4N-WM",serialNumber="SN123456"} 0
	`
)

type TestErrorResponse struct {
	Error TestError `json:"error"`
}

type TestError struct {
	Code         string        `json:"code"`
	Message      string        `json:"message"`
	ExtendedInfo []TestMessage `json:"@Message.ExtendedInfo"`
}

type TestMessage struct {
	MessageId string `json:"MessageId"`
}

func MustMarshal(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func Test_C220_Exporter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redfish/v1/badcred/Managers/CIMC" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write(MustMarshal(TestErrorResponse{
				Error: TestError{
					Code:    "iLO.0.10.ExtendedInfo",
					Message: "See @Message.ExtendedInfo for more information.",
					ExtendedInfo: []TestMessage{
						{
							MessageId: "Base.1.0.NoValidSession",
						},
					},
				},
			}))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Unknown path - please create test case(s) for it"))
	}))
	defer server.Close()

	ctx := context.Background()
	assert := assert.New(t)

	tests := []struct {
		name       string
		uri        string
		metricName string
		metricRef1 string
		metricRef2 string
		payload    []byte
		expected   string
	}{
		{
			name:       "Bad Credentials",
			uri:        "/redfish/v1/badcred",
			metricName: "up",
			metricRef1: "up",
			metricRef2: "up",
			expected:   up2Expected,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var exporter prometheus.Collector
			var err error
			exporter, err = NewExporter(ctx, server.URL, test.uri, "")
			assert.Nil(err)
			assert.NotNil(exporter)

			prometheus.MustRegister(exporter)

			metric := (*exporter.(*Exporter).deviceMetrics)[test.metricRef1]
			m := (*metric)[test.metricRef2]

			assert.Empty(testutil.CollectAndCompare(m, strings.NewReader(test.expected), test.metricName))

			prometheus.Unregister(exporter)
		})
	}
}

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

func Test_C220_Metrics_Handling(t *testing.T) {

	var HealthyStorageControllerMetricsResponse = []byte(`{
		"Drives":	[{
				"@odata.id":	"/redfish/v1/Systems/WZP1111111/Storage/MRAID/Drives/PD-1"
			}],
		"Name":	"MRAID",
		"StorageControllers":	[{
				"MemberId":	"RAID",
				"Model":	"Cisco 12G Modular Raid Controller with 2GB cache (max 16 drives)",
				"Name":	"Cisco 12G Modular Raid Controller with 2GB cache (max 16 drives)",
				"FirmwareVersion":	"555555",
				"Status":	{
					"State":	"Enabled",
					"Health":	"OK",
					"HealthRollup":	"OK"
				}
			}]
		}`)

	var BadStorageControllerMetricsResponse = []byte(`{
		"Drives":	[],
		"Name":	"MRAID",
		"StorageControllers":	[{
				"MemberId":	"RAID",
				"Model":	"Cisco 12G Modular Raid Controller with 2GB cache (max 16 drives)",
				"Name":	"Cisco 12G Modular Raid Controller with 2GB cache (max 16 drives)",
				"FirmwareVersion":	"555555",
				"Status":	{
					"State":	"Enabled",
					"Health":	"BAD",
					"HealthRollup":	"BAD"
				}
			}]
		}`)

	var HealthyFW4_0_4h_MemoryMetricsResponse = []byte(`{
			"SerialNumber":	"SN123456",
			"MemoryDeviceType":	"DDR4",
			"Status":	{
				"State":	"Enabled"
			},
			"Name":	"DIMM_A1",
			"PartNumber":	"36ASF4G72PZ-2G6D1   ",
			"Manufacturer":	"0x2C00",
			"CapacityMiB":	32768
		}`)

	var BadFW4_0_4h_MemoryMetricsResponse = []byte(`{
			"SerialNumber":	"SN123456",
			"MemoryDeviceType":	"DDR4",
			"Status":	{
				"State":	"Bad"
			},
			"Name":	"DIMM_A1",
			"PartNumber":	"36ASF4G72PZ-2G6D1   ",
			"Manufacturer":	"0x2C00",
			"CapacityMiB":	32768
		}`)

	var HealthyFW4_1_2a_MemoryMetricsResponse = []byte(`{
			"SerialNumber":	"SN123456",
			"MemoryDeviceType":	"DDR4",
			"Status":	{
				"State":	"Enabled",
				"Health":	"OK"
			},
			"Name":	"DIMM_A1",
			"PartNumber":	"HMA84GR7DJR4N-WM    ",
			"Manufacturer":	"0xAD00",
			"CapacityMiB":	32768
		}`)

	var BadFW4_1_2a_MemoryMetricsResponse = []byte(`{
			"SerialNumber":	"SN123456",
			"MemoryDeviceType":	"DDR4",
			"Status":	{
				"State":	"Enabled",
				"Health":	"NOPE"
			},
			"Name":	"DIMM_A1",
			"PartNumber":	"HMA84GR7DJR4N-WM    ",
			"Manufacturer":	"0xAD00",
			"CapacityMiB":	32768
		}`)

	var exporter prometheus.Collector

	assert := assert.New(t)

	metrx := NewDeviceMetrics()

	exporter = &Exporter{
		ctx:                 context.Background(),
		host:                "fishymetrics.com",
		biosVersion:         "C220M5.4.0.4i.0.zzzzzzzzz",
		chassisSerialNumber: "SN98765",
		deviceMetrics:       metrx,
	}

	prometheus.MustRegister(exporter)

	memDimmMetrics := func(exp *Exporter, resp []byte) error {
		err := exp.exportMemoryMetrics(resp)
		if err != nil {
			return err
		}
		return nil
	}

	storCtrlMetrics := func(exp *Exporter, resp []byte) error {
		err := exp.exportStorageControllerMetrics(resp)
		if err != nil {
			return err
		}
		return nil
	}

	tests := []struct {
		name       string
		metricName string
		metricRef1 string
		metricRef2 string
		handleFunc func(*Exporter, []byte) error
		response   []byte
		expected   string
	}{
		{
			name:       "Healthy FW 4.0.4h",
			metricName: "c220_storage_controller_status",
			metricRef1: "driveMetrics",
			metricRef2: "storageControllerStatus",
			handleFunc: storCtrlMetrics,
			response:   HealthyStorageControllerMetricsResponse,
			expected:   HealthyStorageControllerMetricsExpected,
		},
		{
			name:       "Healthy FW 4.0.4h",
			metricName: "c220_storage_controller_status",
			metricRef1: "driveMetrics",
			metricRef2: "storageControllerStatus",
			handleFunc: storCtrlMetrics,
			response:   BadStorageControllerMetricsResponse,
			expected:   BadStorageControllerMetricsExpected,
		},
		{
			name:       "Healthy FW 4.0.4h",
			metricName: "c220_memory_dimm_status",
			metricRef1: "memoryMetrics",
			metricRef2: "memoryStatus",
			handleFunc: memDimmMetrics,
			response:   HealthyFW4_0_4h_MemoryMetricsResponse,
			expected:   HealthyFW4_0_4h_MemoryMetricsExpected,
		},
		{
			name:       "Bad FW 4.0.4h",
			metricName: "c220_memory_dimm_status",
			metricRef1: "memoryMetrics",
			metricRef2: "memoryStatus",
			handleFunc: memDimmMetrics,
			response:   BadFW4_0_4h_MemoryMetricsResponse,
			expected:   BadFW4_0_4h_MemoryMetricsExpected,
		},
		{
			name:       "Healthy FW 4.1.2a",
			metricName: "c220_memory_dimm_status",
			metricRef1: "memoryMetrics",
			metricRef2: "memoryStatus",
			handleFunc: memDimmMetrics,
			response:   HealthyFW4_1_2a_MemoryMetricsResponse,
			expected:   HealthyFW4_1_2a_MemoryMetricsExpected,
		},
		{
			name:       "Bad FW 4.1.2a",
			metricName: "c220_memory_dimm_status",
			metricRef1: "memoryMetrics",
			metricRef2: "memoryStatus",
			handleFunc: memDimmMetrics,
			response:   BadFW4_1_2a_MemoryMetricsResponse,
			expected:   BadFW4_1_2a_MemoryMetricsExpected,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.handleFunc(exporter.(*Exporter), test.response)
			if err != nil {
				t.Error(err)
			}

			metric := (*exporter.(*Exporter).deviceMetrics)[test.metricRef1]
			m := (*metric)[test.metricRef2]

			assert.Empty(testutil.CollectAndCompare(m, strings.NewReader(test.expected), test.metricName))

			m.Reset()
		})
	}

	prometheus.Unregister(exporter)
}
