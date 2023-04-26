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
	"bytes"
	"encoding/json"
)

// JSON
// /redfish/v1/Chassis/XXXXX/Power/

// PowerMetrics is the top level json object for Power metadata
type PowerMetrics struct {
	Name          string         `json:"Name"`
	PowerControl  []PowerControl `json:"PowerControl"`
	PowerSupplies []PowerSupply  `json:"PowerSupplies,omitempty"`
	Voltages      []Voltages     `json:"Voltages"`
	Url           string         `json:"@odata.id"`
}

// PowerControl is the top level json object for metadata on power supply consumption
type PowerControl struct {
	PowerLimit         PowerLimitWrapper  `json:"PowerLimit"`
	PowerConsumedWatts int                `json:"PowerConsumedWatts,omitempty"`
	PowerMetrics       PowerMetricWrapper `json:"PowerMetrics"`
}

type PowerLimit struct {
	LimitInWatts int `json:"LimitInWatts,omitempty"`
}

type PowerLimitWrapper struct {
	PowerLimit
}

func (w *PowerLimitWrapper) UnmarshalJSON(data []byte) error {
	if bytes.Compare([]byte("[]"), data) == 0 {
		w.PowerLimit = PowerLimit{}
		return nil
	}
	return json.Unmarshal(data, &w.PowerLimit)
}

// PowerMetric contains avg/min/max power metadata
type PowerMetric struct {
	AverageConsumedWatts int `json:"AverageConsumedWatts"`
	MaxConsumedWatts     int `json:"MaxConsumedWatts"`
	MinConsumedWatts     int `json:"MinConsumedWatts"`
}

type PowerMetricWrapper struct {
	PowerMetric
}

func (w *PowerMetricWrapper) UnmarshalJSON(data []byte) error {
	// because of a bug in the s3260m5 firmware we need to account for this
	if bytes.Compare([]byte("[]"), data) == 0 {
		w.PowerMetric = PowerMetric{}
		return nil
	}
	return json.Unmarshal(data, &w.PowerMetric)
}

// Voltages contains current/lower/upper voltage and power supply status metadata
type Voltages struct {
	Name                   string  `json:"Name"`
	ReadingVolts           float64 `json:"ReadingVolts"`
	Status                 Status  `json:"Status"`
	UpperThresholdCritical float64 `json:"UpperThresholdCritical"`
}

// PowerSupply is the top level json object for metadata on power supply product info
type PowerSupply struct {
	FirmwareVersion      string       `json:"FirmwareVersion"`
	LastPowerOutputWatts int          `json:"LastPowerOutputWatts"`
	LineInputVoltage     string       `json:"LineInputVoltage,omitempty"`
	LineInputVoltageType string       `json:"LineInputVoltageType,omitempty"`
	InputRanges          []InputRange `json:"InputRanges,omitempty"`
	Manufacturer         string       `json:"Manufacturer"`
	Model                string       `json:"Model"`
	Name                 string       `json:"Name"`
	PowerSupplyType      string       `json:"PowerSupplyType"`
	SerialNumber         string       `json:"SerialNumber"`
	SparePartNumber      string       `json:"SparePartNumber,omitempty"`
	Status               Status       `json:"Status"`
}

// InputRange is the top level json object for input voltage metadata
type InputRange struct {
	InputType     string `json:"InputType"`
	OutputWattage int    `json:"OutputWattage"`
}
