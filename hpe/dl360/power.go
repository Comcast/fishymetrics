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

package dl360

// /redfish/v1/Chassis/1/Power/

// PowerMetrics is the top level json object for Power metadata
type PowerMetrics struct {
	ID            string         `json:"Id"`
	Name          string         `json:"Name"`
	PowerControl  []PowerControl `json:"PowerControl"`
	PowerSupplies []PowerSupply  `json:"PowerSupplies"`
}

// PowerControl is the top level json object for metadata on power supply consumption
type PowerControl struct {
	MemberID           string      `json:"MemberId"`
	PowerCapacityWatts int         `json:"PowerCapacityWatts"`
	PowerConsumedWatts int         `json:"PowerConsumedWatts"`
	PowerMetrics       PowerMetric `json:"PowerMetrics"`
}

// PowerMetric contains avg/min/max power metadata
type PowerMetric struct {
	AverageConsumedWatts int `json:"AverageConsumedWatts"`
	IntervalInMin        int `json:"IntervalInMin"`
	MaxConsumedWatts     int `json:"MaxConsumedWatts"`
	MinConsumedWatts     int `json:"MinConsumedWatts"`
}

// PowerSupply is the top level json object for metadata on power supply product info
type PowerSupply struct {
	FirmwareVersion      string   `json:"FirmwareVersion"`
	LastPowerOutputWatts int      `json:"LastPowerOutputWatts"`
	LineInputVoltage     int      `json:"LineInputVoltage"`
	LineInputVoltageType string   `json:"LineInputVoltageType"`
	Manufacturer         string   `json:"Manufacturer"`
	MemberID             string   `json:"MemberId"`
	Model                string   `json:"Model"`
	Name                 string   `json:"Name"`
	Oem                  OemPower `json:"Oem"`
	PowerCapacityWatts   int      `json:"PowerCapacityWatts"`
	PowerSupplyType      string   `json:"PowerSupplyType"`
	SerialNumber         string   `json:"SerialNumber"`
	SparePartNumber      string   `json:"SparePartNumber"`
	Status               Status   `json:"Status"`
}

// OemPower is the top level json object for historical data for wattage
type OemPower struct {
	Hpe Hpe `json:"Hpe"`
	Hp  Hpe `json:"Hp"`
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
