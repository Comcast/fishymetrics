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
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/comcast/fishymetrics/common"
	"github.com/comcast/fishymetrics/oem"
)

func handle(exp *Exporter, metricType ...string) []common.Handler {
	var handlers []common.Handler

	for _, m := range metricType {
		if m == THERMAL {
			handlers = append(handlers, exp.exportThermalMetrics)
		} else if m == POWER {
			handlers = append(handlers, exp.exportPowerMetrics)
		} else if m == NVME {
			handlers = append(handlers, exp.exportNVMeDriveMetrics)
		} else if m == DISKDRIVE {
			handlers = append(handlers, exp.exportPhysicalDriveMetrics)
		} else if m == LOGICALDRIVE {
			handlers = append(handlers, exp.exportLogicalDriveMetrics)
		} else if m == UNKNOWN_DRIVE {
			handlers = append(handlers, exp.exportUnknownDriveMetrics)
		} else if m == STORAGE_CONTROLLER {
			handlers = append(handlers, exp.exportStorageControllerMetrics)
		} else if m == MEMORY {
			handlers = append(handlers, exp.exportMemoryMetrics)
		} else if m == MEMORY_SUMMARY {
			handlers = append(handlers, exp.exportMemorySummaryMetrics)
		} else if m == FIRMWARE {
			handlers = append(handlers, exp.exportFirmwareMetrics)
		} else if m == PROCESSOR {
			handlers = append(handlers, exp.exportProcessorMetrics)
		} else if m == STORAGEBATTERY {
			handlers = append(handlers, exp.exportStorageBattery)
		} else if m == ILOSELFTEST {
			handlers = append(handlers, exp.exportIloSelfTest)
		}
	}

	return handlers
}

// exportFirmwareMetrics collects the device metrics in json format and sets the prometheus gauges
func (e *Exporter) exportFirmwareMetrics(body []byte) error {
	var mgr oem.Manager
	var dm = (*e.DeviceMetrics)["deviceInfo"]
	err := json.Unmarshal(body, &mgr)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling FirmwareMetrics - " + err.Error())
	}

	(*dm)["deviceInfo"].WithLabelValues(mgr.Description, e.ChassisSerialNumber, e.Model, mgr.FirmwareVersion, e.biosVersion).Set(1.0)

	return nil
}

// exportPowerMetrics collects the power metrics in json format and sets the prometheus gauges
func (e *Exporter) exportPowerMetrics(body []byte) error {
	var state float64
	var pm oem.PowerMetrics
	var pow = (*e.DeviceMetrics)["powerMetrics"]
	var bay int
	err := json.Unmarshal(body, &pm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling PowerMetrics - " + err.Error())
	}

	for _, pc := range pm.PowerControl.PowerControl {
		var watts float64
		switch pc.PowerConsumedWatts.(type) {
		case float64:
			if pc.PowerConsumedWatts.(float64) > 0 {
				watts = pc.PowerConsumedWatts.(float64)
			}
		case string:
			if pc.PowerConsumedWatts.(string) != "" {
				watts, _ = strconv.ParseFloat(pc.PowerConsumedWatts.(string), 32)
			}
		default:
			// use the AverageConsumedWatts if PowerConsumedWatts is not present
			switch pc.PowerMetrics.AverageConsumedWatts.(type) {
			case float64:
				watts = pc.PowerMetrics.AverageConsumedWatts.(float64)
			case string:
				watts, _ = strconv.ParseFloat(pc.PowerMetrics.AverageConsumedWatts.(string), 32)
			}
		}
		(*pow)["supplyTotalConsumed"].WithLabelValues(pm.Url, e.ChassisSerialNumber, e.Model).Set(watts)
	}

	for _, pv := range pm.Voltages {
		if pv.Status.State == "Enabled" {
			var volts float64
			switch pv.ReadingVolts.(type) {
			case float64:
				volts = pv.ReadingVolts.(float64)
			case string:
				volts, _ = strconv.ParseFloat(pv.ReadingVolts.(string), 32)
			}
			(*pow)["voltageOutput"].WithLabelValues(pv.Name, e.ChassisSerialNumber, e.Model).Set(volts)
			if pv.Status.Health == "OK" {
				state = OK
			} else {
				state = BAD
			}
		} else {
			state = BAD
		}

		(*pow)["voltageStatus"].WithLabelValues(pv.Name, e.ChassisSerialNumber, e.Model).Set(state)
	}

	for _, ps := range pm.PowerSupplies {
		if ps.Status.State == "Enabled" {
			var watts float64
			switch ps.LastPowerOutputWatts.(type) {
			case float64:
				watts = ps.LastPowerOutputWatts.(float64)
			case string:
				watts, _ = strconv.ParseFloat(ps.LastPowerOutputWatts.(string), 32)
			}

			if ps.Oem.Hp.PowerSupplyStatus.State != "" {
				bay = ps.Oem.Hp.BayNumber
			} else if ps.Oem.Hpe.PowerSupplyStatus.State != "" {
				bay = ps.Oem.Hpe.BayNumber
			}

			(*pow)["supplyOutput"].WithLabelValues(ps.Name, e.ChassisSerialNumber, e.Model, strings.TrimRight(ps.Manufacturer, " "), ps.SerialNumber, ps.FirmwareVersion, ps.PowerSupplyType, strconv.Itoa(bay), ps.Model).Set(watts)
			if ps.Status.Health == "OK" {
				state = OK
			} else if ps.Status.Health == "" {
				state = OK
			} else {
				state = BAD
			}
		} else {
			state = BAD
		}

		(*pow)["supplyStatus"].WithLabelValues(ps.Name, e.ChassisSerialNumber, e.Model, strings.TrimRight(ps.Manufacturer, " "), ps.SerialNumber, ps.FirmwareVersion, ps.PowerSupplyType, strconv.Itoa(bay), ps.Model).Set(state)
	}

	return nil
}

// exportThermalMetrics collects the thermal and fan metrics in json format and sets the prometheus gauges
func (e *Exporter) exportThermalMetrics(body []byte) error {

	var state float64
	var tm oem.ThermalMetrics
	var therm = (*e.DeviceMetrics)["thermalMetrics"]
	err := json.Unmarshal(body, &tm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling ThermalMetrics - " + err.Error())
	}

	if tm.Status.State == "Enabled" {
		if tm.Status.Health == "OK" {
			state = OK
		} else {
			state = BAD
		}
		(*therm)["thermalSummary"].WithLabelValues(tm.Url, e.ChassisSerialNumber, e.Model).Set(state)
	}

	// Iterate through fans
	for _, fan := range tm.Fans {
		// Check fan status and convert string to numeric values
		if fan.Status.State == "Enabled" {
			var fanSpeed float64
			switch fan.Reading.(type) {
			case string:
				fanSpeed, _ = strconv.ParseFloat(fan.Reading.(string), 32)
			case float64:
				fanSpeed = fan.Reading.(float64)
			}

			if fan.FanName != "" {
				(*therm)["fanSpeed"].WithLabelValues(fan.FanName, e.ChassisSerialNumber, e.Model).Set(float64(fan.CurrentReading))
			} else {
				(*therm)["fanSpeed"].WithLabelValues(fan.Name, e.ChassisSerialNumber, e.Model).Set(fanSpeed)
			}
			if fan.Status.Health == "OK" {
				state = OK
			} else {
				state = BAD
			}
			if fan.FanName != "" {
				(*therm)["fanStatus"].WithLabelValues(fan.FanName, e.ChassisSerialNumber, e.Model).Set(state)
			} else {
				(*therm)["fanStatus"].WithLabelValues(fan.Name, e.ChassisSerialNumber, e.Model).Set(state)
			}
		}
	}

	// Iterate through sensors
	for _, sensor := range tm.Temperatures {
		// Check sensor status and convert string to numeric values
		if sensor.Status.State == "Enabled" {
			var celsTemp float64
			switch sensor.ReadingCelsius.(type) {
			case string:
				celsTemp, _ = strconv.ParseFloat(sensor.ReadingCelsius.(string), 32)
			case int:
				celsTemp = float64(sensor.ReadingCelsius.(int))
			case float64:
				celsTemp = sensor.ReadingCelsius.(float64)
			}
			(*therm)["sensorTemperature"].WithLabelValues(strings.TrimRight(sensor.Name, " "), e.ChassisSerialNumber, e.Model).Set(celsTemp)
			if sensor.Status.Health == "OK" {
				state = OK
			} else {
				state = BAD
			}
			(*therm)["sensorStatus"].WithLabelValues(strings.TrimRight(sensor.Name, " "), e.ChassisSerialNumber, e.Model).Set(state)
		}
	}

	return nil
}

// exportPhysicalDriveMetrics collects the physical drive metrics in json format and sets the prometheus gauges
func (e *Exporter) exportPhysicalDriveMetrics(body []byte) error {

	var state float64
	var dlphysical oem.DiskDriveMetrics
	var dlphysicaldrive = (*e.DeviceMetrics)["diskDriveMetrics"]
	var loc string
	var cap int
	err := json.Unmarshal(body, &dlphysical)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling DiskDriveMetrics - " + err.Error())
	}
	// Check physical drive is enabled then check status and convert string to numeric values
	if dlphysical.Status.State == "Enabled" {
		if dlphysical.Status.Health == "OK" {
			state = OK
		} else {
			state = BAD
		}
	} else if dlphysical.Status.Health == "OK" {
		state = OK
	} else {
		state = BAD
	}

	if dlphysical.Location != "" {
		loc = dlphysical.Location
	} else if dlphysical.PhysicalLocation.PartLocation.ServiceLabel != "" {
		loc = dlphysical.PhysicalLocation.PartLocation.ServiceLabel
	}

	if dlphysical.CapacityMiB != 0 {
		cap = dlphysical.CapacityMiB
	} else if dlphysical.CapacityBytes != 0 {
		// convert to MiB
		cap = ((dlphysical.CapacityBytes / 1024) / 1024)
	}

	if dlphysical.Location != "" {
		loc = dlphysical.Location
	} else if dlphysical.PhysicalLocation.PartLocation.ServiceLabel != "" {
		loc = dlphysical.PhysicalLocation.PartLocation.ServiceLabel
	}

	if dlphysical.CapacityMiB != 0 {
		cap = dlphysical.CapacityMiB
	} else if dlphysical.CapacityBytes != 0 {
		// convert to MiB
		cap = ((dlphysical.CapacityBytes / 1024) / 1024)
	}

	// Physical drives need to have a unique identifier like location so as to not overwrite data
	// physical drives can have the same ID, but belong to a different ArrayController, therefore need more than just the ID as a unique identifier.
	(*dlphysicaldrive)["driveStatus"].WithLabelValues(dlphysical.Name, e.ChassisSerialNumber, e.Model, dlphysical.Id, loc, dlphysical.SerialNumber, strconv.Itoa(cap)).Set(state)
	return nil
}

// exportLogicalDriveMetrics collects the logical drive metrics in json format and sets the prometheus gauges
func (e *Exporter) exportLogicalDriveMetrics(body []byte) error {
	var state float64
	var dllogical oem.LogicalDriveMetrics
	var dllogicaldrive = (*e.DeviceMetrics)["logicalDriveMetrics"]
	err := json.Unmarshal(body, &dllogical)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling LogicalDriveMetrics - " + err.Error())
	}
	// Check physical drive is enabled then check status and convert string to numeric values
	if dllogical.Status.State == "Enabled" {
		if dllogical.Status.Health == "OK" {
			state = OK
		} else {
			state = BAD
		}
	} else {
		state = DISABLED
	}

	(*dllogicaldrive)["raidStatus"].WithLabelValues(dllogical.Name, e.ChassisSerialNumber, e.Model, dllogical.LogicalDriveName, dllogical.VolumeUniqueIdentifier, dllogical.Raid).Set(state)
	return nil
}

// exportNVMeDriveMetrics collects the NVME drive metrics in json format and sets the prometheus gauges
func (e *Exporter) exportNVMeDriveMetrics(body []byte) error {
	var state float64
	var dlnvme oem.NVMeDriveMetrics
	var dlnvmedrive = (*e.DeviceMetrics)["nvmeMetrics"]
	err := json.Unmarshal(body, &dlnvme)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling NVMeDriveMetrics - " + err.Error())
	}

	// Check nvme drive is enabled then check status and convert string to numeric values
	if dlnvme.Oem.Hpe.DriveStatus.State == "Enabled" {
		if dlnvme.Oem.Hpe.DriveStatus.Health == "OK" {
			state = OK
		} else {
			state = BAD
		}
	} else {
		state = DISABLED
	}

	(*dlnvmedrive)["nvmeDriveStatus"].WithLabelValues(e.ChassisSerialNumber, e.Model, dlnvme.Protocol, dlnvme.ID, dlnvme.PhysicalLocation.PartLocation.ServiceLabel).Set(state)
	return nil
}

// exportUnknownDriveMetrics figured out what protocol the drive is using and then determines which handler to use for metrics collection
func (e *Exporter) exportUnknownDriveMetrics(body []byte) error {

	var protocol oem.DriveProtocol
	err := json.Unmarshal(body, &protocol)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling for drive protocol - " + err.Error())
	}

	if protocol.Protocol == "NVMe" {
		err = e.exportNVMeDriveMetrics(body)
		if err != nil {
			return fmt.Errorf("Error Unmarshalling NVMeDriveMetrics - " + err.Error())
		}
	} else if protocol.Protocol != "" {
		err = e.exportPhysicalDriveMetrics(body)
		if err != nil {
			return fmt.Errorf("Error Unmarshalling DiskDriveMetrics - " + err.Error())
		}
	}

	return nil
}

// exportStorageControllerMetrics collects the raid controller metrics in json format and sets the prometheus gauges
func (e *Exporter) exportStorageControllerMetrics(body []byte) error {

	var state float64
	var scm oem.StorageControllerMetrics
	var drv = (*e.DeviceMetrics)["storageCtrlMetrics"]
	err := json.Unmarshal(body, &scm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling StorageControllerMetrics - " + err.Error())
	}

	for _, sc := range scm.StorageController.StorageController {
		if sc.Status.State == "Enabled" {
			if sc.Status.Health == "OK" {
				state = OK
			} else {
				state = BAD
			}
			(*drv)["storageControllerStatus"].WithLabelValues(scm.Name, e.ChassisSerialNumber, e.Model, sc.FirmwareVersion, sc.Model).Set(state)
		}
	}

	return nil
}

// exportMemorySummaryMetrics collects the memory summary metrics in json format and sets the prometheus gauges
func (e *Exporter) exportMemorySummaryMetrics(body []byte) error {

	var state float64
	var dlm oem.System
	var dlMemory = (*e.DeviceMetrics)["memoryMetrics"]
	err := json.Unmarshal(body, &dlm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling MemorySummaryMetrics - " + err.Error())
	}
	// Check memory status and convert string to numeric values
	if dlm.MemorySummary.Status.HealthRollup == "OK" {
		state = OK
	} else {
		state = BAD
	}

	(*dlMemory)["memoryStatus"].WithLabelValues(e.ChassisSerialNumber, e.Model, strconv.Itoa(dlm.MemorySummary.TotalSystemMemoryGiB)).Set(state)

	return nil
}

// exportStorageBattery collects the smart storge battery metrics in json format and sets the prometheus guage
func (e *Exporter) exportStorageBattery(body []byte) error {

	var state float64
	var chasStorBatt oem.ChassisStorageBattery
	var storBattery = (*e.DeviceMetrics)["storBatteryMetrics"]
	err := json.Unmarshal(body, &chasStorBatt)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling Storage Battery Metrics - " + err.Error())
	}

	if fmt.Sprint(chasStorBatt.Oem.Hp.Battery) != "null" && len(chasStorBatt.Oem.Hp.Battery) > 0 {
		for _, ssbat := range chasStorBatt.Oem.Hp.Battery {
			if ssbat.Present == "Yes" {
				if ssbat.Condition == "Ok" {
					state = OK
				} else {
					state = BAD
				}
				(*storBattery)["storageBatteryStatus"].WithLabelValues(strconv.Itoa(ssbat.Index), e.ChassisSerialNumber, e.Model, strings.TrimRight(ssbat.Name, " "), ssbat.Model).Set(state)
			}
		}
	} else if fmt.Sprint(chasStorBatt.Oem.Hpe.Battery) != "null" && len(chasStorBatt.Oem.Hpe.Battery) > 0 {
		for _, ssbat := range chasStorBatt.Oem.Hpe.Battery {
			if ssbat.Present == "Yes" {
				if ssbat.Condition == "Ok" {
					state = OK
				} else {
					state = BAD
				}
				(*storBattery)["storageBatteryStatus"].WithLabelValues(strconv.Itoa(ssbat.Index), e.ChassisSerialNumber, e.Model, strings.TrimRight(ssbat.Name, " "), ssbat.Model).Set(state)
			}
		}
	} else if len(chasStorBatt.Oem.Hpe.BatteryAlt) > 0 {
		for _, ssbat := range chasStorBatt.Oem.Hpe.BatteryAlt {
			if ssbat.Status.State == "Enabled" {
				if ssbat.Status.Health == "OK" {
					state = OK
				} else {
					state = BAD
				}
				(*storBattery)["storageBatteryStatus"].WithLabelValues(strconv.Itoa(ssbat.Index), e.ChassisSerialNumber, e.Model, strings.TrimRight(ssbat.Name, " "), ssbat.Model).Set(state)
			}
		}
	} else if len(chasStorBatt.Oem.Hp.BatteryAlt) > 0 {
		for _, ssbat := range chasStorBatt.Oem.Hp.BatteryAlt {
			if ssbat.Status.State == "Enabled" {
				if ssbat.Status.Health == "OK" {
					state = OK
				} else {
					state = BAD
				}
				(*storBattery)["storageBatteryStatus"].WithLabelValues(strconv.Itoa(ssbat.Index), e.ChassisSerialNumber, e.Model, strings.TrimRight(ssbat.Name, " "), ssbat.Model).Set(state)
			}
		}
	}

	return nil
}

// exportMemoryMetrics collects the memory dimm metrics in json format and sets the prometheus gauges
func (e *Exporter) exportMemoryMetrics(body []byte) error {

	var state float64
	var memCap string
	var mm oem.MemoryMetrics
	var mem = (*e.DeviceMetrics)["memoryMetrics"]
	err := json.Unmarshal(body, &mm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling MemoryMetrics - " + err.Error())
	}

	if mm.DIMMStatus != "" {
		switch mm.SizeMB.(type) {
		case string:
			memCap = mm.SizeMB.(string)
		case int:
			memCap = strconv.Itoa(mm.SizeMB.(int))
		case float64:
			memCap = strconv.Itoa(int(mm.SizeMB.(float64)))
		}

		if mm.DIMMStatus == "GoodInUse" {
			state = OK
		} else {
			state = BAD
		}

		(*mem)["memoryDimmStatus"].WithLabelValues(mm.Name, e.ChassisSerialNumber, e.Model, memCap, strings.TrimRight(mm.Manufacturer, " "), strings.TrimRight(mm.PartNumber, " ")).Set(state)
	} else if mm.Status != "" {
		var status string

		switch mm.CapacityMiB.(type) {
		case string:
			memCap = mm.CapacityMiB.(string)
		case int:
			memCap = strconv.Itoa(mm.CapacityMiB.(int))
		case float64:
			memCap = strconv.Itoa(int(mm.CapacityMiB.(float64)))
		}

		switch mm.Status.(type) {
		case string:
			status = mm.Status.(string)
			if status == "Operable" {
				state = OK
			} else {
				state = BAD
			}
		default:
			if s, ok := mm.Status.(map[string]interface{}); ok {
				switch s["State"].(type) {
				case string:
					if s["State"].(string) == "Enabled" {
						switch s["Health"].(type) {
						case string:
							if s["Health"].(string) == "OK" {
								state = OK
							} else if s["Health"].(string) == "" {
								state = OK
							} else {
								state = BAD
							}
						case nil:
							state = OK
						}
					} else if s["State"].(string) == "Absent" {
						return nil
					} else {
						state = BAD
					}
				}
			}
		}
		(*mem)["memoryDimmStatus"].WithLabelValues(mm.Name, e.ChassisSerialNumber, e.Model, memCap, strings.TrimRight(mm.Manufacturer, " "), strings.TrimRight(mm.PartNumber, " ")).Set(state)
	}

	return nil
}

// exportProcessorMetrics collects the processor metrics in json format and sets the prometheus gauges
func (e *Exporter) exportProcessorMetrics(body []byte) error {

	var state float64
	var totCores string
	var pm oem.ProcessorMetrics
	var proc = (*e.DeviceMetrics)["processorMetrics"]
	err := json.Unmarshal(body, &pm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling ProcessorMetrics - " + err.Error())
	}

	switch pm.TotalCores.(type) {
	case string:
		totCores = pm.TotalCores.(string)
	case float64:
		totCores = strconv.Itoa(int(pm.TotalCores.(float64)))
	case int:
		totCores = strconv.Itoa(pm.TotalCores.(int))
	}
	if pm.Status.Health == "OK" {
		state = OK
	} else {
		state = BAD
	}
	(*proc)["processorStatus"].WithLabelValues(pm.Id, e.ChassisSerialNumber, e.Model, pm.Socket, pm.Model, totCores).Set(state)

	return nil
}

// exportIloSelfTest collects the iLO Self Test Results metrics in json format and sets the prometheus guage
func (e *Exporter) exportIloSelfTest(body []byte) error {

	var state float64
	var sysm oem.ChassisStorageBattery
	var iloSelfTst = (*e.DeviceMetrics)["iloSelfTestMetrics"]
	err := json.Unmarshal(body, &sysm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling iLO Self Test Metrics - " + err.Error())
	}

	if fmt.Sprint(sysm.Oem.Hp.IloSelfTest) != "null" && len(sysm.Oem.Hp.IloSelfTest) > 0 {
		for _, ilost := range sysm.Oem.Hp.IloSelfTest {
			if ilost.Status != "Informational" {
				if ilost.Status == "OK" {
					state = OK
				} else {
					state = BAD
				}
				(*iloSelfTst)["iloSelfTestStatus"].WithLabelValues(ilost.Name, e.ChassisSerialNumber, e.Model).Set(state)
			}
		}
	} else if fmt.Sprint(sysm.Oem.Hpe.IloSelfTest) != "null" && len(sysm.Oem.Hpe.IloSelfTest) > 0 {
		for _, ilost := range sysm.Oem.Hpe.IloSelfTest {
			if ilost.Status != "Informational" {
				if ilost.Status == "OK" {
					state = OK
				} else {
					state = BAD
				}
				(*iloSelfTst)["iloSelfTestStatus"].WithLabelValues(ilost.Name, e.ChassisSerialNumber, e.Model).Set(state)
			}
		}
	}

	return nil
}
