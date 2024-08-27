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
			"fanSpeed":          newServerMetric("redfish_thermal_fan_speed", "Current fan speed in the unit of percentage, possible values are 0 - 100", nil, []string{"name", "chassisSerialNumber", "chassisModel"}),
			"fanStatus":         newServerMetric("redfish_thermal_fan_status", "Current fan status 1 = OK, 0 = BAD", nil, []string{"name", "chassisSerialNumber", "chassisModel"}),
			"sensorTemperature": newServerMetric("redfish_thermal_sensor_temperature", "Current sensor temperature reading in Celsius", nil, []string{"name", "chassisSerialNumber", "chassisModel"}),
			"sensorStatus":      newServerMetric("redfish_thermal_sensor_status", "Current sensor status 1 = OK, 0 = BAD", nil, []string{"name", "chassisSerialNumber", "chassisModel"}),
			"thermalSummary":    newServerMetric("redfish_thermal_summary_status", "Current sensor status 1 = OK, 0 = BAD", nil, []string{"url", "chassisSerialNumber", "chassisModel"}),
		}

		PowerMetrics = &metrics{
			"voltageOutput":       newServerMetric("redfish_power_voltage_output", "Power voltage output in watts", nil, []string{"name", "chassisSerialNumber", "chassisModel"}),
			"voltageStatus":       newServerMetric("redfish_power_voltage_status", "Current power voltage status 1 = OK, 0 = BAD", nil, []string{"name", "chassisSerialNumber", "chassisModel"}),
			"supplyOutput":        newServerMetric("redfish_power_supply_output", "Power supply output in watts", nil, []string{"name", "chassisSerialNumber", "chassisModel", "manufacturer", "serialNumber", "firmwareVersion", "powerSupplyType", "bayNumber", "model"}),
			"supplyStatus":        newServerMetric("redfish_power_supply_status", "Current power supply status 1 = OK, 0 = BAD", nil, []string{"name", "chassisSerialNumber", "chassisModel", "manufacturer", "serialNumber", "firmwareVersion", "powerSupplyType", "bayNumber", "model"}),
			"supplyTotalConsumed": newServerMetric("redfish_power_supply_total_consumed", "Total output of all power supplies in watts", nil, []string{"memberId", "chassisSerialNumber", "chassisModel"}),
		}

		ProcessorMetrics = &metrics{
			"processorStatus": newServerMetric("redfish_cpu_status", "Current cpu status 1 = OK, 0 = BAD", nil, []string{"id", "chassisSerialNumber", "chassisModel", "socket", "model", "totalCores"}),
		}

		IloSelfTestMetrics = &metrics{
			"iloSelfTestStatus": newServerMetric("redfish_ilo_selftest_status", "Current ilo selftest status 1 = OK, 0 = BAD", nil, []string{"name", "chassisSerialNumber", "chassisModel"}),
		}

		StorageBatteryMetrics = &metrics{
			"storageBatteryStatus": newServerMetric("redfish_storage_battery_status", "Current storage battery status 1 = OK, 0 = BAD", nil, []string{"id", "chassisSerialNumber", "chassisModel", "name", "model"}),
		}

		// Splitting out the three different types of drives to gather metrics on each (NVMe, Disk Drive, and Logical Drive)
		// NVMe Drive Metrics
		NVMeDriveMetrics = &metrics{
			"nvmeDriveStatus": newServerMetric("redfish_nvme_drive_status", "Current NVME status 1 = OK, 0 = BAD, -1 = DISABLED", nil, []string{"chassisSerialNumber", "chassisModel", "protocol", "id", "serviceLabel"}),
		}

		// Physical Storage Disk Drive Metrics
		DiskDriveMetrics = &metrics{
			"driveStatus": newServerMetric("redfish_disk_drive_status", "Current Disk Drive status 1 = OK, 0 = BAD, -1 = DISABLED", nil, []string{"name", "chassisSerialNumber", "chassisModel", "id", "location", "serialnumber", "capacityMiB"}),
		}

		// Logical Disk Drive Metrics
		LogicalDriveMetrics = &metrics{
			"raidStatus": newServerMetric("redfish_logical_drive_status", "Current Logical Drive Raid 1 = OK, 0 = BAD, -1 = DISABLED", nil, []string{"name", "chassisSerialNumber", "chassisModel", "logicaldrivename", "volumeuniqueidentifier", "raid"}),
		}

		StorageControllerMetrics = &metrics{
			"storageControllerStatus": newServerMetric("redfish_storage_controller_status", "Current storage controller status 1 = OK, 0 = BAD", nil, []string{"name", "chassisSerialNumber", "chassisModel", "firmwareVersion", "model", "location"}),
		}

		MemoryMetrics = &metrics{
			"memoryStatus":     newServerMetric("redfish_memory_status", "Current memory status 1 = OK, 0 = BAD", nil, []string{"chassisSerialNumber", "chassisModel", "totalSystemMemoryGiB"}),
			"memoryDimmStatus": newServerMetric("redfish_memory_dimm_status", "Current dimm status 1 = OK, 0 = BAD", nil, []string{"name", "chassisSerialNumber", "chassisModel", "capacityMiB", "manufacturer", "partNumber"}),
		}

		// Component Firmware Metrics
		FirmwareInventoryMetrics = &metrics{
			"componentFirmware": newServerMetric("redfish_component_firmware", "Current firmware component status 1 = OK, 0 = BAD", nil, []string{"id", "name", "description", "version"}),
		}

		DeviceMetrics = &metrics{
			"deviceInfo": newServerMetric("redfish_device_info", "Current snapshot of device firmware information", nil, []string{"name", "chassisSerialNumber", "chassisModel", "firmwareVersion", "biosVersion"}),
		}

		Metrics = &map[string]*metrics{
			"up":                       UpMetric,
			"thermalMetrics":           ThermalMetrics,
			"powerMetrics":             PowerMetrics,
			"processorMetrics":         ProcessorMetrics,
			"nvmeMetrics":              NVMeDriveMetrics,
			"diskDriveMetrics":         DiskDriveMetrics,
			"logicalDriveMetrics":      LogicalDriveMetrics,
			"storBatteryMetrics":       StorageBatteryMetrics,
			"storageCtrlMetrics":       StorageControllerMetrics,
			"iloSelfTestMetrics":       IloSelfTestMetrics,
			"firmwareInventoryMetrics": FirmwareInventoryMetrics,
			"memoryMetrics":            MemoryMetrics,
			"deviceInfo":               DeviceMetrics,
		}
	)

	return Metrics
}
