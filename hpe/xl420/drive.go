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

// NVME's
// /redfish/v1/chassis/1/
type NVMeDriveMetrics struct {
	ID               string           `json:"Id"`
	Model            string           `json:"Model"`
	Name             string           `json:"Name"`
	MediaType        string           `json:"MediaType"`
	Oem              Oem              `json:"Oem"`
	PhysicalLocation PhysicalLocation `json:"PhysicalLocation"`
	Protocol         string           `json:"Protocol"`
	Status           Status           `json:"Status"`
	FailurePredicted bool             `json:"FailurePredicted"`
	CapacityBytes    int              `json:"CapacityBytes"`
}

// Logical Drives
// /redfish/v1/Systems/1/SmartStorage/ArrayControllers/X/LogicalDrives/X/
type LogicalDriveMetrics struct {
	Id                     string `json:"Id"`
	CapacityMiB            int    `json:"CapacityMiB"`
	Description            string `json:"Description"`
	InterfaceType          string `json:"InterfaceType"`
	LogicalDriveName       string `json:"LogicalDriveName"`
	LogicalDriveNumber     int    `json:"LogicalDriveNumber"`
	Name                   string `json:"Name"`
	Raid                   string `json:"Raid"`
	Status                 Status `json:"Status"`
	StripeSizebytes        int    `json:"StripeSizebytes"`
	VolumeUniqueIdentifier string `json:"VolumeUniqueIdentifier"`
}

// Disk Drives
// /redfish/v1/Systems/1/SmartStorage/ArrayControllers/X/DiskDrives/X/
type DiskDriveMetrics struct {
	Id            string `json:"Id"`
	CapacityMiB   int    `json:"CapacityMiB"`
	Description   string `json:"Description"`
	InterfaceType string `json:"InterfaceType"`
	Name          string `json:"Name"`
	Model         string `json:"Model"`
	Status        Status `json:"Status"`
	Location      string `json:"Location"`
	SerialNumber  string `json:"SerialNumber"`
}

// GenericDrive is used to iterate over differing drive endpoints
// /redfish/v1/Systems/1/SmartStorage/ArrayControllers/ for Logical and Physical Drives
// /redfish/v1/Chassis/1/Drives/ for NVMe Drive(s)
type GenericDrive struct {
	Members      []Members  `json:"Members,omitempty"`
	LinksUpper   LinksUpper `json:"Links,omitempty"`
	LinksLower   LinksLower `json:"links,omitempty"`
	MembersCount int        `json:"Members@odata.count,omitempty"`
}

type Members struct {
	URL string `json:"@odata.id"`
}

type LinksUpper struct {
	Drives         []URL `json:"Drives,omitempty"`
	LogicalDrives  URL   `json:"LogicalDrives,omitempty"`
	PhysicalDrives URL   `json:"PhysicalDrives,omitempty"`
}

type LinksLower struct {
	Drives         []HRef `json:"Drives,omitempty"`
	LogicalDrives  HRef   `json:"LogicalDrives,omitempty"`
	PhysicalDrives HRef   `json:"PhysicalDrives,omitempty"`
}

type HRef struct {
	URL string `json:"href"`
}

type URL struct {
	URL string `json:"@odata.id"`
}

// PhysicalLocation
type PhysicalLocation struct {
	PartLocation PartLocation `json:"PartLocation"`
}

// PartLocation is a variable that determines the Box and the Bay location of the NVMe drive
type PartLocation struct {
	ServiceLabel string `json:"ServiceLabel"`
}

// Contents of Oem
type Oem struct {
	Hpe HpeCont `json:"Hpe"`
}

// Contents of Hpe
type HpeCont struct {
	CurrentTemperatureCelsius int    `json:"CurrentTemperatureCelsius"`
	DriveStatus               Status `json:"DriveStatus"`
	NVMeID                    string `json:"NVMeId"`
}
