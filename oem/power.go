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

package oem

import (
	"encoding/json"
)

// /redfish/v1/Chassis/X/Power/

// PowerMetrics is the top level json object for Power metadata
type PowerMetrics struct {
	ID            string              `json:"Id"`
	Name          string              `json:"Name"`
	PowerControl  PowerControlWrapper `json:"PowerControl"`
	PowerSupplies []PowerSupply       `json:"PowerSupplies"`
	Voltages      []Voltages          `json:"Voltages,omitempty"`
	Url           string              `json:"@odata.id"`
}

// PowerControl is the top level json object for metadata on power supply consumption
type PowerControl struct {
	MemberID           string      `json:"MemberId"`
	PowerCapacityWatts interface{} `json:"PowerCapacityWatts,omitempty"`
	PowerConsumedWatts interface{} `json:"PowerConsumedWatts"`
	PowerMetrics       PowerMetric `json:"PowerMetrics"`
}

type PowerControlSlice struct {
	PowerControl []PowerControl
}

type PowerControlWrapper struct {
	PowerControlSlice
}

func (w *PowerControlWrapper) UnmarshalJSON(data []byte) error {
	// because of a change in output between firmware versions we need to account for this
	// PowerControl can either be []PowerControl or a singular PowerControl
	var powCtl PowerControl
	err := json.Unmarshal(data, &powCtl)
	if err == nil {
		s := make([]PowerControl, 0)
		s = append(s, powCtl)
		w.PowerControl = s
	} else {
		return json.Unmarshal(data, &w.PowerControl)
	}

	return nil
}

// PowerMetric contains avg/min/max power metadata
type PowerMetric struct {
	AverageConsumedWatts interface{} `json:"AverageConsumedWatts"`
	IntervalInMin        interface{} `json:"IntervalInMin,omitempty"`
	MaxConsumedWatts     interface{} `json:"MaxConsumedWatts"`
	MinConsumedWatts     interface{} `json:"MinConsumedWatts"`
}

// PowerSupply is the top level json object for metadata on power supply product info
type PowerSupply struct {
	FirmwareVersion      string       `json:"FirmwareVersion"`
	LastPowerOutputWatts interface{}  `json:"LastPowerOutputWatts"`
	LineInputVoltage     interface{}  `json:"LineInputVoltage"`
	LineInputVoltageType string       `json:"LineInputVoltageType,omitempty"`
	InputRanges          []InputRange `json:"InputRanges,omitempty"`
	Manufacturer         string       `json:"Manufacturer"`
	MemberID             interface{}  `json:"MemberId"`
	Model                string       `json:"Model"`
	Name                 string       `json:"Name"`
	Oem                  OemPower     `json:"Oem,omitempty"`
	PowerCapacityWatts   int          `json:"PowerCapacityWatts,omitempty"`
	PowerSupplyType      string       `json:"PowerSupplyType"`
	SerialNumber         string       `json:"SerialNumber"`
	SparePartNumber      string       `json:"SparePartNumber"`
	Status               Status       `json:"Status"`
}

// InputRange is the top level json object for input voltage metadata
type InputRange struct {
	InputType     string `json:"InputType,omitempty"`
	OutputWattage int    `json:"OutputWattage"`
}

// OemPower is the top level json object for historical data for wattage
type OemPower struct {
	Hpe Hpe `json:"Hpe,omitempty"`
	Hp  Hpe `json:"Hp,omitempty"`
}

// Hpe contains metadata on power supply product info
type Hpe struct {
	AveragePowerOutputWatts int    `json:"AveragePowerOutputWatts"`
	BayNumber               int    `json:"BayNumber"`
	HotplugCapable          bool   `json:"HotplugCapable"`
	MaxPowerOutputWatts     int    `json:"MaxPowerOutputWatts"`
	Mismatched              bool   `json:"Mismatched"`
	PowerSupplyStatus       Status `json:"PowerSupplyStatus"`
	IPDUCapable             bool   `json:"iPDUCapable"`
}

// Voltages contains current/lower/upper voltage and power supply status metadata
type Voltages struct {
	Name                   string      `json:"Name"`
	ReadingVolts           interface{} `json:"ReadingVolts"`
	Status                 Status      `json:"Status"`
	UpperThresholdCritical interface{} `json:"UpperThresholdCritical"`
}
