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

	"github.com/comcast/fishymetrics/oem"
)

// exportFirmwareMetrics collects the device metrics in json format and sets the prometheus gauges
func (e *Exporter) exportFirmwareMetrics(body []byte) error {
	var chas oem.Chassis
	var dm = (*e.deviceMetrics)["deviceInfo"]
	err := json.Unmarshal(body, &chas)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling FirmwareMetrics - " + err.Error())
	}

	(*dm)["deviceInfo"].WithLabelValues(chas.Description, e.chassisSerialNumber, e.model, chas.FirmwareVersion, e.biosVersion).Set(1.0)

	return nil
}

// exportPowerMetrics collects the power metrics in json format and sets the prometheus gauges
func (e *Exporter) exportPowerMetrics(body []byte) error {

	var state float64
	var pm oem.PowerMetrics
	var pow = (*e.deviceMetrics)["powerMetrics"]
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
		(*pow)["supplyTotalConsumed"].WithLabelValues(pc.MemberID, e.chassisSerialNumber, e.model).Set(watts)
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
			(*pow)["voltageOutput"].WithLabelValues(pv.Name, e.chassisSerialNumber, e.model).Set(volts)
			if pv.Status.Health == "OK" {
				state = OK
			} else {
				state = BAD
			}
		} else {
			state = BAD
		}

		(*pow)["voltageStatus"].WithLabelValues(pv.Name, e.chassisSerialNumber, e.model).Set(state)
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
			(*pow)["supplyOutput"].WithLabelValues(ps.Name, e.chassisSerialNumber, e.model, ps.Manufacturer, ps.SparePartNumber, ps.SerialNumber, ps.PowerSupplyType, ps.Model).Set(watts)
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

		(*pow)["supplyStatus"].WithLabelValues(ps.Name, e.chassisSerialNumber, e.model, ps.Manufacturer, ps.SparePartNumber, ps.SerialNumber, ps.PowerSupplyType, ps.Model).Set(state)
	}

	return nil
}

// exportThermalMetrics collects the thermal and fan metrics in json format and sets the prometheus gauges
func (e *Exporter) exportThermalMetrics(body []byte) error {

	var state float64
	var tm oem.ThermalMetrics
	var therm = (*e.deviceMetrics)["thermalMetrics"]
	err := json.Unmarshal(body, &tm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling ThermalMetrics - " + err.Error())
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
				(*therm)["fanSpeed"].WithLabelValues(fan.FanName, e.chassisSerialNumber, e.model).Set(float64(fan.CurrentReading))
			} else {
				(*therm)["fanSpeed"].WithLabelValues(fan.Name, e.chassisSerialNumber, e.model).Set(fanSpeed)
			}
			if fan.Status.Health == "OK" {
				state = OK
			} else {
				state = BAD
			}
			if fan.FanName != "" {
				(*therm)["fanStatus"].WithLabelValues(fan.FanName, e.chassisSerialNumber, e.model).Set(state)
			} else {
				(*therm)["fanStatus"].WithLabelValues(fan.Name, e.chassisSerialNumber, e.model).Set(state)
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
			(*therm)["sensorTemperature"].WithLabelValues(strings.TrimRight(sensor.Name, " "), e.chassisSerialNumber, e.model).Set(celsTemp)
			if sensor.Status.Health == "OK" {
				state = OK
			} else {
				state = BAD
			}
			(*therm)["sensorStatus"].WithLabelValues(strings.TrimRight(sensor.Name, " "), e.chassisSerialNumber, e.model).Set(state)
		}
	}

	return nil
}

// exportPhysicalDriveMetrics collects the physical drive metrics in json format and sets the prometheus gauges
func (e *Exporter) exportPhysicalDriveMetrics(body []byte) error {

	var state float64
	var dlphysical oem.DiskDriveMetrics
	var dlphysicaldrive = (*e.deviceMetrics)["diskDriveMetrics"]
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
	} else {
		state = DISABLED
	}

	// Physical drives need to have a unique identifier like location so as to not overwrite data
	// physical drives can have the same ID, but belong to a different ArrayController, therefore need more than just the ID as a unique identifier.
	(*dlphysicaldrive)["driveStatus"].WithLabelValues(dlphysical.Name, e.chassisSerialNumber, e.model, dlphysical.Id, dlphysical.Location, dlphysical.SerialNumber).Set(state)
	return nil
}

// exportLogicalDriveMetrics collects the logical drive metrics in json format and sets the prometheus gauges
func (e *Exporter) exportLogicalDriveMetrics(body []byte) error {
	var state float64
	var dllogical oem.LogicalDriveMetrics
	var dllogicaldrive = (*e.deviceMetrics)["logicalDriveMetrics"]
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

	(*dllogicaldrive)["raidStatus"].WithLabelValues(dllogical.Name, e.chassisSerialNumber, e.model, dllogical.LogicalDriveName, dllogical.VolumeUniqueIdentifier, dllogical.Raid).Set(state)
	return nil
}

// exportNVMeDriveMetrics collects the NVME drive metrics in json format and sets the prometheus gauges
func (e *Exporter) exportNVMeDriveMetrics(body []byte) error {
	var state float64
	var dlnvme oem.NVMeDriveMetrics
	var dlnvmedrive = (*e.deviceMetrics)["nvmeMetrics"]
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

	(*dlnvmedrive)["nvmeDriveStatus"].WithLabelValues(e.chassisSerialNumber, e.model, dlnvme.Protocol, dlnvme.ID, dlnvme.PhysicalLocation.PartLocation.ServiceLabel).Set(state)
	return nil
}

// exportMemorySummaryMetrics collects the memory summary metrics in json format and sets the prometheus gauges
func (e *Exporter) exportMemorySummaryMetrics(body []byte) error {

	var state float64
	var dlm oem.MemorySummaryMetrics
	var dlMemory = (*e.deviceMetrics)["memoryMetrics"]
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

	(*dlMemory)["memoryStatus"].WithLabelValues(e.chassisSerialNumber, e.model, strconv.Itoa(dlm.MemorySummary.TotalSystemMemoryGiB)).Set(state)

	return nil
}

// exportStorageBattery collects the smart storge battery metrics in json format and sets the prometheus guage
func (e *Exporter) exportStorageBattery(body []byte) error {

	var state float64
	var sysm oem.SystemMetrics
	var storBattery = (*e.deviceMetrics)["storBatteryMetrics"]
	err := json.Unmarshal(body, &sysm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling Storage Battery Metrics - " + err.Error())
	}

	if fmt.Sprint(sysm.Oem.Hp.Battery) != "null" && len(sysm.Oem.Hp.Battery) > 0 {
		for _, ssbat := range sysm.Oem.Hp.Battery {
			if ssbat.Present == "Yes" {
				if ssbat.Condition == "Ok" {
					state = OK
				} else {
					state = BAD
				}
				(*storBattery)["storageBatteryStatus"].WithLabelValues(strconv.Itoa(ssbat.Index), e.chassisSerialNumber, e.model, ssbat.Name, ssbat.Model, ssbat.SerialNumber).Set(state)
			}
		}
	} else if fmt.Sprint(sysm.Oem.Hpe.Battery) != "null" && len(sysm.Oem.Hpe.Battery) > 0 {
		for _, ssbat := range sysm.Oem.Hpe.Battery {
			if ssbat.Present == "Yes" {
				if ssbat.Condition == "Ok" {
					state = OK
				} else {
					state = BAD
				}
				(*storBattery)["storageBatteryStatus"].WithLabelValues(strconv.Itoa(ssbat.Index), e.chassisSerialNumber, e.model, ssbat.Name, ssbat.Model, ssbat.SerialNumber).Set(state)
			}
		}
	}

	return nil
}

// exportMemoryMetrics collects the memory dimm metrics in json format and sets the prometheus gauges
func (e *Exporter) exportMemoryMetrics(body []byte) error {

	var state float64
	var mm oem.MemoryMetrics
	var mem = (*e.deviceMetrics)["memoryMetrics"]
	err := json.Unmarshal(body, &mm)
	if err != nil {
		return fmt.Errorf("Error Unmarshalling MemoryMetrics - " + err.Error())
	}

	if mm.Status != "" {
		var memCap string
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
		(*mem)["memoryDimmStatus"].WithLabelValues(mm.Name, e.chassisSerialNumber, e.model, memCap, mm.Manufacturer, strings.TrimRight(mm.PartNumber, " "), mm.SerialNumber).Set(state)
	}

	return nil
}

// exportProcessorMetrics collects the processor metrics in json format and sets the prometheus gauges
func (e *Exporter) exportProcessorMetrics(body []byte) error {

	var state float64
	var totCores string
	var pm oem.ProcessorMetrics
	var proc = (*e.deviceMetrics)["processorMetrics"]
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
	(*proc)["processorStatus"].WithLabelValues(pm.Id, e.chassisSerialNumber, e.model, pm.Socket, pm.Model, totCores).Set(state)

	return nil
}

// exportIloSelfTest collects the iLO Self Test Results metrics in json format and sets the prometheus guage
func (e *Exporter) exportIloSelfTest(body []byte) error {

	var state float64
	var sysm oem.SystemMetrics
	var iloSelfTst = (*e.deviceMetrics)["iloSelfTestMetrics"]
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
				(*iloSelfTst)["iloSelfTestStatus"].WithLabelValues(ilost.Name, e.chassisSerialNumber, e.model).Set(state)
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
				(*iloSelfTst)["iloSelfTestStatus"].WithLabelValues(ilost.Name, e.chassisSerialNumber, e.model).Set(state)
			}
		}
	}

	return nil
}
