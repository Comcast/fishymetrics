// /*
//  * Copyright 2023 Comcast Cable Communications Management, LLC
//  *
//  * Licensed under the Apache License, Version 2.0 (the "License");
//  * you may not use this file except in compliance with the License.
//  * You may obtain a copy of the License at
//  *
//  *     http://www.apache.org/licenses/LICENSE-2.0
//  *
//  * Unless required by applicable law or agreed to in writing, software
//  * distributed under the License is distributed on an "AS IS" BASIS,
//  * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  * See the License for the specific language governing permissions and
//  * limitations under the License.
//  */

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
	dimmHeader = `
		 # HELP c220_memory_dimm_status Current dimm status 1 = OK, 0 = BAD
		 # TYPE c220_memory_dimm_status gauge
 `
)

func Test_C220_Memory_Metrics(t *testing.T) {

	var exporter prometheus.Collector

	assert := assert.New(t)

	HealthyFW4_0_4h_MemoryMetricsResponse, _ := json.Marshal(MemoryMetrics{
		SerialNumber:     "SN123456",
		MemoryDeviceType: "DDR4",
		PartNumber:       "36ASF4G72PZ-2G6D1   ",
		CapacityMiB:      32768,
		Name:             "DIMM_A1",
		Manufacturer:     "0x2C00",
		Status: Status{
			State: "Enabled",
		},
	})

	BadFW4_0_4h_MemoryMetricsResponse, _ := json.Marshal(MemoryMetrics{
		SerialNumber:     "SN123456",
		MemoryDeviceType: "DDR4",
		PartNumber:       "36ASF4G72PZ-2G6D1   ",
		CapacityMiB:      32768,
		Name:             "DIMM_A1",
		Manufacturer:     "0x2C00",
		Status: Status{
			State: "Bad",
		},
	})

	HealthyFW4_1_2a_MemoryMetricsResponse, _ := json.Marshal(MemoryMetrics{
		SerialNumber:     "SN123456",
		MemoryDeviceType: "DDR4",
		PartNumber:       "HMA84GR7DJR4N-WM    ",
		CapacityMiB:      32768,
		Name:             "DIMM_A1",
		Manufacturer:     "0xAD00",
		Status: Status{
			State:  "Enabled",
			Health: "OK",
		},
	})

	BadFW4_1_2a_MemoryMetricsResponse, _ := json.Marshal(MemoryMetrics{
		SerialNumber:     "SN123456",
		MemoryDeviceType: "DDR4",
		PartNumber:       "HMA84GR7DJR4N-WM    ",
		CapacityMiB:      32768,
		Name:             "DIMM_A1",
		Manufacturer:     "0xAD00",
		Status: Status{
			State:  "Enabled",
			Health: "NOPE",
		},
	})

	metrx := NewDeviceMetrics()

	exporter = &Exporter{
		ctx:                 context.Background(),
		host:                "fishymetrics.com",
		biosVersion:         "C220M5.4.0.4i.0.0831191119",
		chassisSerialNumber: "SN78901",
		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "up",
			Help: "Was the last scrape of chassis monitor successful.",
		}),
		deviceMetrics: metrx,
	}

	prometheus.MustRegister(exporter)

	tests := []struct {
		name     string
		response []byte
		expected string
	}{
		{
			name:     "Healthy FW 4.0.4h",
			response: HealthyFW4_0_4h_MemoryMetricsResponse,
			expected: `
			c220_memory_dimm_status{capacityMiB="32768",chassisSerialNumber="SN78901",manufacturer="0x2C00",name="DIMM_A1",partNumber="36ASF4G72PZ-2G6D1",serialNumber="SN123456"} 1
			`,
		},
		{
			name:     "Bad FW 4.0.4h",
			response: BadFW4_0_4h_MemoryMetricsResponse,
			expected: `
			c220_memory_dimm_status{capacityMiB="32768",chassisSerialNumber="SN78901",manufacturer="0x2C00",name="DIMM_A1",partNumber="36ASF4G72PZ-2G6D1",serialNumber="SN123456"} 0
			`,
		},
		{
			name:     "Healthy FW 4.1.2a",
			response: HealthyFW4_1_2a_MemoryMetricsResponse,
			expected: `
			c220_memory_dimm_status{capacityMiB="32768",chassisSerialNumber="SN78901",manufacturer="0xAD00",name="DIMM_A1",partNumber="HMA84GR7DJR4N-WM",serialNumber="SN123456"} 1
			`,
		},
		{
			name:     "Bad FW 4.1.2a",
			response: BadFW4_1_2a_MemoryMetricsResponse,
			expected: `
			c220_memory_dimm_status{capacityMiB="32768",chassisSerialNumber="SN78901",manufacturer="0xAD00",name="DIMM_A1",partNumber="HMA84GR7DJR4N-WM",serialNumber="SN123456"} 0
			`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := exporter.(*Exporter).exportMemoryMetrics(tt.response)
			if err != nil {
				t.Error(err)
			}

			mem := (*exporter.(*Exporter).deviceMetrics)["memoryMetrics"]
			memMetrics := (*mem)["memoryStatus"]

			assert.Empty(testutil.CollectAndCompare(memMetrics, strings.NewReader(dimmHeader+tt.expected), "c220_memory_dimm_status"))

			memMetrics.Reset()

		})
	}

	prometheus.Unregister(exporter)
}
