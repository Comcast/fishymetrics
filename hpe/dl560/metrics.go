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

package dl560

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
		UpMetric = &metrics{
			"up": newServerMetric("up", "was the last scrape of fishymetrics successful.", nil, []string{}),
		}

		ThermalMetrics = &metrics{
			"fanSpeed":          newServerMetric("dl560_thermal_fan_speed", "Current fan speed in the unit of percentage, possible values are 0 - 100", nil, []string{"name", "chassisSerialNumber"}),
			"fanStatus":         newServerMetric("dl560_thermal_fan_status", "Current fan status 1 = OK, 0 = BAD", nil, []string{"name", "chassisSerialNumber"}),
			"sensorTemperature": newServerMetric("dl560_thermal_sensor_temperature", "Current sensor temperature reading in Celsius", nil, []string{"name", "chassisSerialNumber"}),
			"sensorStatus":      newServerMetric("dl560_thermal_sensor_status", "Current sensor status 1 = OK, 0 = BAD", nil, []string{"name", "chassisSerialNumber"}),
		}

		PowerMetrics = &metrics{
			"voltageOutput":       newServerMetric("dl560_power_voltage_output", "Power voltage output in watts", nil, []string{"name", "chassisSerialNumber"}),
			"voltageStatus":       newServerMetric("dl560_power_voltage_status", "Current power voltage status 1 = OK, 0 = BAD", nil, []string{"name", "chassisSerialNumber"}),
			"supplyOutput":        newServerMetric("dl560_power_supply_output", "Power supply output in watts", nil, []string{"name", "chassisSerialNumber", "manufacturer", "partNumber", "serialNumber", "powerSupplyType", "model"}),
			"supplyStatus":        newServerMetric("dl560_power_supply_status", "Current power supply status 1 = OK, 0 = BAD", nil, []string{"name", "chassisSerialNumber", "manufacturer", "partNumber", "serialNumber", "powerSupplyType", "model"}),
			"supplyTotalConsumed": newServerMetric("dl560_power_supply_total_consumed", "Total output of all power supplies in watts", nil, []string{"memberId", "chassisSerialNumber"}),
		}

		// Splitting out the three different types of drives to gather metrics on each (NVMe, Disk Drive, and Logical Drive)
		// NVMe Drive Metrics
		NVMeDriveMetrics = &metrics{
			"nvmeDriveStatus": newServerMetric("dl560_nvme_drive_status", "Current NVME status 1 = OK, 0 = BAD, -1 = DISABLED", nil, []string{"chassisSerialNumber", "protocol", "id", "serviceLabel"}),
		}

		// Phyiscal Storage Disk Drive Metrics
		DiskDriveMetrics = &metrics{
			"driveStatus": newServerMetric("dl560_disk_drive_status", "Current Disk Drive status 1 = OK, 0 = BAD, -1 = DISABLED", nil, []string{"name", "chassisSerialNumber", "id", "location", "serialnumber"}),
		}

		// Logical Disk Drive Metrics
		LogicalDriveMetrics = &metrics{
			"raidStatus": newServerMetric("dl560_logical_drive_status", "Current Logical Drive Raid 1 = OK, 0 = BAD, -1 = DISABLED", nil, []string{"name", "chassisSerialNumber", "logicaldrivename", "volumeuniqueidentifier", "raid"}),
		}

		MemoryMetrics = &metrics{
			"memoryStatus":     newServerMetric("dl560_memory_status", "Current memory status 1 = OK, 0 = BAD", nil, []string{"chassisSerialNumber", "totalSystemMemoryGiB"}),
			"memoryDimmStatus": newServerMetric("dl560_memory_dimm_status", "Current dimm status 1 = OK, 0 = BAD", nil, []string{"name", "chassisSerialNumber", "capacityMiB", "manufacturer", "partNumber", "serialNumber"}),
		}

		DeviceMetrics = &metrics{
			"deviceInfo": newServerMetric("device_info", "Current snapshot of device firmware information", nil, []string{"name", "chassisSerialNumber", "firmwareVersion", "biosVersion", "model"}),
		}

		Metrics = &map[string]*metrics{
			"up":                  UpMetric,
			"thermalMetrics":      ThermalMetrics,
			"powerMetrics":        PowerMetrics,
			"nvmeMetrics":         NVMeDriveMetrics,
			"diskDriveMetrics":    DiskDriveMetrics,
			"logicalDriveMetrics": LogicalDriveMetrics,
			"memoryMetrics":       MemoryMetrics,
			"deviceInfo":          DeviceMetrics,
		}
	)

	return Metrics
}
