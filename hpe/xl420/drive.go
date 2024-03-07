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

package xl420

// /redfish/v1/Systems/1/SmartStorage/ArrayControllers/

type GenericDrive struct {
	Members []struct {
		URL string `json:"@odata.id"`
	} `json:"Members"`
	MembersCount int `json:"Members@odata.count,omitempty"`
	Links        *struct {
		LogicalDrives struct {
			URL string `json:"href"`
		} `json:"LogicalDrives,omitempty"`
		PhysicalDrives struct {
			URL string `json:"href"`
		} `json:"PhysicalDrives,omitempty"`
	} `json:"Links,omitempty"`
	Link *struct {
		LogicalDrives struct {
			URL string `json:"href"`
		} `json:"LogicalDrives,omitempty"`
		PhysicalDrives struct {
			URL string `json:"href"`
		} `json:"PhysicalDrives,omitempty"`
	} `json:"links,omitempty"`
}

// /redfish/v1/Systems/1/SmartStorage/ArrayControllers/X/LogicalDrives/X/
type LogicalDriveMetrics struct {
	ID                 string `json:"Id"`
	CapacityMiB        int    `json:"CapacityMiB"`
	Description        string `json:"Description"`
	InterfaceType      string `json:"InterfaceType"`
	LogicalDriveName   string `json:"LogicalDriveName"`
	LogicalDriveNumber int    `json:"LogicalDriveNumber"`
	Name               string `json:"Name"`
	Raid               string `json:"Raid"`
	Status             Status `json:"Status"`
	StripeSizeBytes    int    `json:"StripeSizeBytes"`
}

// /redfish/v1/Systems/1/SmartStorage/ArrayControllers/X/DiskDrives/X/
type PhysicalDriveMetrics struct {
	ID           string `json:"Id"`
	CapacityGB   int    `json:"CapacityGB"`
	Location     string `json:"Location"`
	Model        string `json:"Model"`
	Name         string `json:"Name"`
	SerialNumber string `json:"SerialNumber"`
	Status       Status `json:"Status"`
}
