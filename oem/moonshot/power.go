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

package moonshot

// /rest/v1/Chassis/1/PowerMetrics

// PowerMetrics is the top level json object for Power metadata
type PowerMetrics struct {
	Name               string          `json:"Name"`
	Oem                OemPower        `json:"Oem"`
	PowerCapacityWatts int             `json:"PowerCapacityWatts"`
	PowerConsumedWatts int             `json:"PowerConsumedWatts"`
	PowerSupplies      []PowerSupplies `json:"PowerSupplies"`
	Type               string          `json:"Type"`
	Links              Links           `json:"links"`
}

// OemPower is the top level json object for historical data for wattage
type OemPower struct {
	Hp HpPower `json:"Hp"`
}

// HpPower is the top level json object for the power supplies metadata
type HpPower struct {
	PowercapDescription string                `json:"PowercapDescription"`
	PowercapMode        int                   `json:"PowercapMode"`
	InstantWattage      int                   `json:"InstantWattage,omitempty"`
	Type                string                `json:"Type"`
	WattageHistoryLevel []WattageHistoryLevel `json:"WattageHistoryLevel"`
}

// WattageHistoryLevel is the top level json object for all historical Samples metadata
type WattageHistoryLevel struct {
	Counter    int            `json:"Counter"`
	Cumulator  int            `json:"Cumulator"`
	SampleType string         `json:"SampleType"`
	Samples    []SamplesPower `json:"Samples"`
}

// SamplesPower holds the historical data for power wattage
type SamplesPower struct {
	Wattage string `json:"Wattage"`
}

// StatusPower is the variable to determine if a power supply is OK or not
type StatusPower struct {
	State string `json:"State"`
}

// PowerSupplies is the top level json object for metadata on power supply product info
type PowerSupplies struct {
	ACInputStatus        string      `json:"ACInputStatus"`
	FirmwareVersion      string      `json:"FirmwareVersion"`
	LastPowerOutputWatts int         `json:"LastPowerOutputWatts"`
	Model                string      `json:"Model"`
	Name                 string      `json:"Name"`
	Oem                  Oem         `json:"Oem"`
	PowerCapacityWatts   int         `json:"PowerCapacityWatts"`
	PowerSupplyType      string      `json:"PowerSupplyType"`
	SerialNumber         string      `json:"SerialNumber"`
	SparePartNumber      string      `json:"SparePartNumber"`
	Status               StatusPower `json:"Status"`
}

// Oem namespace layer for Hp json object
type Oem struct {
	Hp Hp `json:"Hp"`
}

// Hp contains metadata on power supply product info
type Hp struct {
	BayNumber      int    `json:"BayNumber"`
	HotplugCapable bool   `json:"HotplugCapable"`
	Type           string `json:"Type"`
}

// Links is a reference to the current REST API URL call
type Links struct {
	Self struct {
		Href string `json:"href"`
	} `json:"self"`
}
