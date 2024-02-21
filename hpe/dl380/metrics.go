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

package dl380

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
			"fanSpeed":          newServerMetric("dl380_thermal_fan_speed", "Current fan speed in the unit of percentage, possible values are 0 - 100", nil, []string{"name"}),
			"fanStatus":         newServerMetric("dl380_thermal_fan_status", "Current fan status 1 = OK, 0 = BAD", nil, []string{"name"}),
			"sensorTemperature": newServerMetric("dl380_thermal_sensor_temperature", "Current sensor temperature reading in Celsius", nil, []string{"name"}),
			"sensorStatus":      newServerMetric("dl380_thermal_sensor_status", "Current sensor status 1 = OK, 0 = BAD", nil, []string{"name"}),
		}

		PowerMetrics = &metrics{
			"supplyOutput":        newServerMetric("dl380_power_supply_output", "Power supply output in watts", nil, []string{"memberId", "sparePartNumber"}),
			"supplyStatus":        newServerMetric("dl380_power_supply_status", "Current power supply status 1 = OK, 0 = BAD", nil, []string{"memberId", "sparePartNumber"}),
			"supplyTotalConsumed": newServerMetric("dl380_power_supply_total_consumed", "Total output of all power supplies in watts", nil, []string{"memberId"}),
			"supplyTotalCapacity": newServerMetric("dl380_power_supply_total_capacity", "Total output capacity of all the power supplies", nil, []string{"memberId"}),
		}

		// Splitting out the three different types of drives to gather metrics on each (NVMe, Disk Drive, and Logical Drive)
		// NVMe Drive Metrics
		NVMeDriveMetrics = &metrics{
			"nvmeDriveStatus": newServerMetric("dl380_nvme_drive_status", "Current NVME status 1 = OK, 0 = BAD", nil, []string{"protocol", "id", "serviceLabel"}),
		}

		// Phyiscal Storage Disk Drive Metrics
		DiskDriveMetrics = &metrics{
			"driveStatus": newServerMetric("dl380_disk_drive_status", "Current Disk Drive status 1 = OK, 0 = BAD", nil, []string{"name", "Id", "location"}), // DiskDriveStatus values
		}

		// Logical Disk Drive Metrics
		LogicalDriveMetrics = &metrics{
			"raidStatus": newServerMetric("dl380_logical_drive_raid", "Current Logical Drive Raid", nil, []string{"name", "logicaldrivename", "volumeuniqueidentifier", "raid"}), // Logical Drive Raid value
		}

		MemoryMetrics = &metrics{
			"memoryStatus": newServerMetric("dl380_memory_status", "Current memory status 1 = OK, 0 = BAD", nil, []string{"totalSystemMemoryGiB"}),
		}

		Metrics = &map[string]*metrics{
			"thermalMetrics":      ThermalMetrics,
			"powerMetrics":        PowerMetrics,
			"nvmeMetrics":         NVMeDriveMetrics,
			"diskDriveMetrics":    DiskDriveMetrics,
			"logicalDriveMetrics": LogicalDriveMetrics,
			"memoryMetrics":       MemoryMetrics,
		}
	)
	return Metrics
}
