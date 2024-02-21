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
	"github.com/prometheus/client_golang/prometheus"
)

type metrics map[string]*prometheus.GaugeVec

func newServerMetric(metricName string, docString string, constLabels prometheus.Labels, labelNames []string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:        metricName,
			Help:        docString,
			ConstLabels: constLabels,
		},
		labelNames,
	)
}

func NewDeviceMetrics() *map[string]*metrics {
	var (
		ThermalMetrics = &metrics{
			"fanSpeed":          newServerMetric("s3260m5_thermal_fan_speed", "Current fan speed in the unit of RPM", nil, []string{"name", "chassisSerialNumber"}),
			"fanStatus":         newServerMetric("s3260m5_thermal_fan_status", "Current fan status 1 = OK, 0 = BAD", nil, []string{"name", "chassisSerialNumber"}),
			"sensorTemperature": newServerMetric("s3260m5_thermal_sensor_temperature", "Current sensor temperature reading in Celsius", nil, []string{"name", "chassisSerialNumber"}),
			"sensorStatus":      newServerMetric("s3260m5_thermal_sensor_status", "Current sensor status 1 = OK, 0 = BAD", nil, []string{"name", "chassisSerialNumber"}),
			"thermalSummary":    newServerMetric("s3260m5_thermal_summary_status", "Current sensor status 1 = OK, 0 = BAD", nil, []string{"url", "chassisSerialNumber"}),
		}

		PowerMetrics = &metrics{
			"voltageOutput":       newServerMetric("s3260m5_power_voltage_output", "Power voltage output in watts", nil, []string{"name", "chassisSerialNumber"}),
			"voltageStatus":       newServerMetric("s3260m5_power_voltage_status", "Current power voltage status 1 = OK, 0 = BAD", nil, []string{"name", "chassisSerialNumber"}),
			"supplyOutput":        newServerMetric("s3260m5_power_supply_output", "Power supply output in watts", nil, []string{"name", "chassisSerialNumber", "manufacturer", "serialNumber", "firmwareVersion", "powerSupplyType", "model"}),
			"supplyStatus":        newServerMetric("s3260m5_power_supply_status", "Current power supply status 1 = OK, 0 = BAD", nil, []string{"name", "chassisSerialNumber", "manufacturer", "serialNumber", "firmwareVersion", "powerSupplyType", "model"}),
			"supplyTotalConsumed": newServerMetric("s3260m5_power_supply_total_consumed", "Total output of all power supplies in watts", nil, []string{"url", "chassisSerialNumber"}),
		}

		MemoryMetrics = &metrics{
			"memoryStatus": newServerMetric("s3260m5_memory_dimm_status", "Current dimm status 1 = OK, 0 = BAD", nil, []string{"name", "chassisSerialNumber", "capacityMiB", "manufacturer", "partNumber", "serialNumber"}),
		}

		ProcessorMetrics = &metrics{
			"processorStatus": newServerMetric("s3260m5_cpu_status", "Current cpu status 1 = OK, 0 = BAD", nil, []string{"name", "chassisSerialNumber", "description", "totalThreads"}),
		}

		DriveMetrics = &metrics{
			"storageControllerStatus": newServerMetric("s3260m5_storage_controller_status", "Current storage controller status 1 = OK, 0 = BAD, -1 = DISABLED", nil, []string{"name", "chassisSerialNumber", "firmwareVersion", "manufacturer", "model"}),
		}

		DeviceMetrics = &metrics{
			"deviceInfo": newServerMetric("device_info", "Current snapshot of device firmware information", nil, []string{"description", "chassisSerialNumber", "firmwareVersion", "biosVersion", "model"}),
		}

		Metrics = &map[string]*metrics{
			"thermalMetrics":   ThermalMetrics,
			"powerMetrics":     PowerMetrics,
			"memoryMetrics":    MemoryMetrics,
			"processorMetrics": ProcessorMetrics,
			"driveMetrics":     DriveMetrics,
			"deviceInfo":       DeviceMetrics,
		}
	)

	return Metrics
}
