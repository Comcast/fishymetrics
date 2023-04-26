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

// /redfish/v1/Managers/BMC1

// Chassis contains the Model Number, Firmware, etc of the chassis
type Chassis struct {
	FirmwareVersion string `json:"FirmwareVersion"`
	Links           struct {
		ServerManager []struct {
			URL string `json:"@odata.id"`
		} `json:"ManagerForServers"`
	} `json:"Links"`
	Model       string `json:"Model"`
	Description string `json:"Description"`
}

// Collection returns an array of the endpoints from the chassis pertaining to a resource type
type Collection struct {
	Members []struct {
		URL string `json:"@odata.id"`
	} `json:"Members"`
	MembersCount int `json:"Members@odata.count"`
}

// Status contains metadata for the health of a particular component/module
type Status struct {
	Health string `json:"Health,omitempty"`
	State  string `json:"State,omitempty"`
}

// /redfish/v1/Systems/XXXXX

// ServerManager contains the BIOS version of the chassis
type ServerManager struct {
	BiosVersion string `json:"BiosVersion"`
}

// /redfish/v1/Chassis/CMC

// Chassis contains the Model Number, Firmware, etc of the chassis
type ChassisSerialNumber struct {
	SerialNumber string `json:"SerialNumber"`
}
