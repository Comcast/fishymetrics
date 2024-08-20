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

// /redfish/v1/Systems/XXXXX

// ServerManager contains the BIOS version and Serial number of the chassis,
// we will also collect memory summary and storage battery metrics if present
type System struct {
	BiosVersion       string        `json:"BiosVersion"`
	SerialNumber      string        `json:"SerialNumber"`
	SystemHostname    string        `json:"HostName"`
	Oem               OemSys        `json:"Oem"`
	MemorySummary     MemorySummary `json:"MemorySummary"`
	Memory            Link          `json:"Memory"`
	Volumes           LinksWrapper  `json:"Volumes"`
	FirmwareInventory LinksWrapper  `json:"FirmwareInventory"`
	Storage           Link          `json:"Storage"`
	UpdateService     Link          `json:"UpdateService"`
}

type OemSys struct {
	Hpe HpeSys `json:"Hpe,omitempty"`
	Hp  HpeSys `json:"Hp,omitempty"`
}

type HpeSys struct {
	Battery     []StorageBattery      `json:"Battery"`
	BatteryAlt  []SmartStorageBattery `json:"SmartStorageBattery"`
	IloSelfTest []IloSelfTest         `json:"iLOSelfTestResults"`
	Links       SystemLinksUpper      `json:"Links"`
	LinksLower  SystemLinksLower      `json:"links"`
}

type SystemLinksUpper struct {
	SmartStorage      Link `json:"SmartStorage"`
	FirmwareInventory Link `json:"FirmwareInventory"`
	Memory            Link `json:"Memory"`
}

type SystemLinksLower struct {
	SmartStorage      HRef `json:"SmartStorage"`
	FirmwareInventory HRef `json:"FirmwareInventory"`
	Memory            HRef `json:"Memory"`
}

type SmartStorageBattery struct {
	Index  int    `json:"Index"`
	Model  string `json:"Model"`
	Status Status `json:"Status"`
	Name   string `json:"ProductName"`
}

type StorageBattery struct {
	Condition string `json:"Condition"`
	Index     int    `json:"Index"`
	Model     string `json:"Model"`
	Present   string `json:"Present"`
	Name      string `json:"ProductName"`
}

// MemorySummary is the json object for MemorySummary metadata
type MemorySummary struct {
	Status                         StatusMemory `json:"Status"`
	TotalSystemMemoryGiB           interface{}  `json:"TotalSystemMemoryGiB"`
	TotalSystemPersistentMemoryGiB int          `json:"TotalSystemPersistentMemoryGiB"`
}

// StatusMemory is the variable to determine if the memory is OK or not
type StatusMemory struct {
	HealthRollup string `json:"HealthRollup"`
	Health       string `json:"Health"`
}
