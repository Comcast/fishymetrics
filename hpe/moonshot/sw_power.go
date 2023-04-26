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

package moonshot

// /rest/v1/chassis/1/switches/sa/PowerMetrics

// SwPowerMetrics is the top level json object for Power metadata
type SwPowerMetrics struct {
	Name          string          `json:"Name"`
	Oem           SwOemPower      `json:"Oem"`
	PowerSupplies []PowerSupplies `json:"PowerSupplies"`
	Type          string          `json:"Type"`
	Links         Links           `json:"links"`
}

// SwOemPower is the top level json object for historical data for wattage
type SwOemPower struct {
	Hp SwHpPower `json:"Hp"`
}

// SwHpPower is the top level json object for the power supplies metadata
type SwHpPower struct {
	InstantWattage      int                     `json:"InstantWattage"`
	MaximumWattage      int                     `json:"MaximumWattage"`
	Type                string                  `json:"Type"`
	WattageHistoryLevel []SwWattageHistoryLevel `json:"WattageHistoryLevel"`
}

// SwWattageHistoryLevel is the top level json object for all historical Samples metadata
type SwWattageHistoryLevel struct {
	Counter    int              `json:"Counter"`
	Cumulator  int              `json:"Cumulator"`
	SampleType string           `json:"SampleType"`
	Samples    []SwSamplesPower `json:"Samples"`
}

// SwSamplesPower holds the historical data for power wattage
type SwSamplesPower struct {
	Wattage string `json:"Wattage"`
}
