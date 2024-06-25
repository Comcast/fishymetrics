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

type GenericFirmware struct {
	Name        string `json:"Name,omitempty"`
	Version     string `json:"Version,VersionString,omitempty"`
	Location    string `json:"Location,omitempty"`
	Description string `json:"Description,omitempty"`
	Status      Status
}

// Collection returns the component details
// /redfish/v1/UpdateService/FirmwareInventory/XXXX/
// type FirmwareComponent struct {
// 	Name        string `json:"Name,omitempty"`
// 	Description string `json:"Description,omitempty"`
// 	Version     string `json:"Version,omitempty"`
// 	Id          string `json:"Id,omitempty"`
// 	Status      Status
// }

type FirmwareComponent struct {
	GenericFirmware
	FirmwareSystemInventory
}

type FirmwareInventory struct {
	Components []FirmwareComponent `json:"Components,omitempty"`
}

// /redfish/v1/Systems/XXXX/FirmwareInventory
type FirmwareSystemInventory struct {
	Current Current `json:"Current,omitempty"`
	Status  Status
}

type Current struct {
	Component Component `json:"Component,omitempty"`
}

type Component struct {
	Details Details `json:"Details,omitempty"`
}

// type Details struct {
// 	Location      string `json:"Location,omitempty"`
// 	Name          string `json:"Name,omitempty"`
// 	VersionString string `json:"VersionString,omitempty"`
// }

type Details struct {
	GenericFirmware
}
