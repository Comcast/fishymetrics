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

package dl380

// /redfish/v1/Systems/1/BaseNetworkAdapters

// NetworkAdapter is the top level json object for DL380 Network Adapter metadata
type NetworkAdapter struct {
	ID             string          `json:"Id"`
	Firmware       Firmware        `json:"Firmware"`
	Name           string          `json:"Name"`
	PartNumber     string          `json:"PartNumber"`
	PhysicalPorts  []PhysicalPorts `json:"PhysicalPorts"`
	SerialNumber   string          `json:"SerialNumber"`
	StructuredName string          `json:"StructuredName"`
	Status         Status          `json:"Status"`
	UEFIDevicePath string          `json:"UEFIDevicePath"`
}

// Firmware is the top level json object for DL380 Network Adapter metadata
type Firmware struct {
	Current FirmwareCurrent `json:"Current"`
}

// FirmwareCurrent contains the version in string format
type FirmwareCurrent struct {
	Version string `json:"VersionString"`
}

// PhysicalPorts contains the metadata for the Chassis NICs
type PhysicalPorts struct {
	FullDuplex    bool   `json:"FullDuplex"`
	IPv4Addresses []Addr `json:"IPv4Addresses"`
	IPv6Addresses []Addr `json:"IPv6Addresses"`
	LinkStatus    string `json:"LinkStatus"`
	MacAddress    string `json:"MacAddress"`
	Name          string `json:"Name"`
	SpeedMbps     int    `json:"SpeedMbps"`
	Status        Status `json:"Status"`
}

// Addr contains the IPv4 or IPv6 Address in string format
type Addr struct {
	Address string `json:"Address"`
}

// Status contains metadata for the health of a particular component/module
type Status struct {
	Health string `json:"Health"`
	State  string `json:"State,omitempty"`
}
