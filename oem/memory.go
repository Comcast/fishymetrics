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

// /redfish/v1/Systems/XXXXX/Memory/DIMM_X1

// MemoryMetrics is the top level json object for a Memory DIMMs metadata
type MemoryMetrics struct {
	Name             string      `json:"Name"`
	CapacityMiB      interface{} `json:"CapacityMiB"`
	Manufacturer     string      `json:"Manufacturer"`
	MemoryDeviceType string      `json:"MemoryDeviceType"`
	PartNumber       string      `json:"PartNumber"`
	SerialNumber     string      `json:"SerialNumber"`
	Status           interface{} `json:"Status"`
}

// /redfish/v1/systems/1/

// MemorySummaryMetrics is the top level json object for all Memory DIMMs metadata
type MemorySummaryMetrics struct {
	ID            string        `json:"Id"`
	MemorySummary MemorySummary `json:"MemorySummary"`
}

// MemorySummary is the json object for MemorySummary metadata
type MemorySummary struct {
	Status                         StatusMemory `json:"Status"`
	TotalSystemMemoryGiB           int          `json:"TotalSystemMemoryGiB"`
	TotalSystemPersistentMemoryGiB int          `json:"TotalSystemPersistentMemoryGiB"`
}

// StatusMemory is the variable to determine if the memory is OK or not
type StatusMemory struct {
	HealthRollup string `json:"HealthRollup"`
}
