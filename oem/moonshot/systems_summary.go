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

// /rest/v1/SystemsSummary

// SystemsSummary is the top level json object for Moonshot summary metadata
type SystemsSummary struct {
	Name             string    `json:"Name"`
	Systems          []Systems `json:"Systems,omitempty"`
	SystemsInChassis int       `json:"SystemsInChassis,omitempty"`
	Type             string    `json:"Type"`
}

// Systems contains metadata on each cartridge that is present in the Moonshot Chassis
type Systems struct {
	AssetTag              string   `json:"AssetTag,omitempty"`
	Health                string   `json:"Health,omitempty"`
	HostMACAddress        []string `json:"HostMACAddress,omitempty"`
	Memory                string   `json:"Memory,omitempty"`
	Model                 string   `json:"Model,omitempty"`
	Name                  string   `json:"Name,omitempty"`
	Power                 string   `json:"Power,omitempty"`
	ProcessorFamily       string   `json:"ProcessorFamily,omitempty"`
	ProcessorManufacturer string   `json:"ProcessorManufacturer,omitempty"`
	SKU                   string   `json:"SKU,omitempty"`
	SerialNumber          string   `json:"SerialNumber,omitempty"`
	UUID                  string   `json:"UUID,omitempty"`
}
