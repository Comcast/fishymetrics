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

package dl380

// NVME's
// /redfish/v1/chassis/1/

// NVMeMetrics is the top level json object for DL380 NVMe Metrics Metadata
// TODO: Ensure Physical Location maps to the ServiceLabel string within PartLocation
// TODO: Ensure Status maps to the Health string within StatusNVMe
type NVMeDriveMetrics struct {
	ID               string          `json:"Id"`
	Model            string          `json:"Model"`
	Name             string          `json:"Name"`
	MediaType        string          `json:"MediaType"`
	PhysicalLocation PartLocation    `json:"PhysicalLocation"`
	Protocol         string          `json:"Protocol"`
	Status           nvmeDriveStatus `json:"Status"`
	FailurePredicted bool            `json:"FailurePredicted"`
	CapacityBytes    int             `json:"CapacityBytes"`
}

// PartLocation is a variable that determines the Box and the Bay location of the NVMe drive
type PartLocation struct {
	ServiceLabel string `json:"ServiceLabel"`
}

// Contents of Oem
type Oem struct {
	Hpe    HpeCont `json:"Hpe"`
	NVMeID string  `json:"NVMeId"`
}

// Contents of Hpe
type HpeCont struct {
	CurrentTemperatureCelsius int             `json:"CurrentTemperatureCelsius"`
	DriveStatus               nvmeDriveStatus `json:"nvmeDriveStatus"`
}

// Status/Health for the NVMe drive
type nvmeDriveStatus struct {
	Health string `json:"Health"`
	State  string `json:"State"`
}

// Smart Array Drives
// /redfish/v1/Systems/1/SmartStorage/ArrayControllers/
// Loop through "Members" [] for each Array controller
//    Example Member: "@odata.id": "/redfish/v1/Systems/1/SmartStorage/ArrayControllers/0/"
//	  Check at the ArrayController Member level to see if there is anything in "Links":"LogicalDrives" -> If so, this is where the LOGICAL DRIVE info can be found, else, continue on to "DiskDrives"
//    If a member is present in /redfish/v1/Systems/1/SmartStorage/ArrayControllers/<member>/LogicalDrives/ , LOOP through the "Members" list and follow that data link
//			Example Memeber: "@odata.id": "/redfish/v1/Systems/1/SmartStorage/ArrayControllers/3/LogicalDrives/1/"

// (only iterate through this if the member count of is > 0.)
// Logical Drives
// TODO: Make sure Status maps to Health string in LogicalDriveStatus
type LogicalDriveMetrics struct {
	Id                 string             `json:"Id"`
	CapacityMiB        int                `json:"CapacityMiB"`
	Description        string             `json:"Description"`
	InterfaceType      string             `json:"InterfaceType"`
	LogicalDriveName   string             `json:"LogicalDriveName"`
	LogicalDriveNumber int                `json:"LogicalDriveNumber"`
	Name               string             `json:"Name"`
	Raid               string             `json:"Raid"`
	Status             LogicalDriveStatus `json:"Status"`
	StripeSizebytes    int                `json:"StripeSizebytes"`
}

// Logical Drive Status
type LogicalDriveStatus struct {
	Health string `json:"Health"`
	State  string `json:"Enabled"`
}

// (Always iterate through this /DiskDrives)
// Disk Drives
// TODO: Make sure Status maps to Health string in DiskDriveStatus
type DiskDriveMetrics struct {
	Id            string          `json:"Id"`
	CapacityMiB   int             `json:"CapacityMiB"`
	Description   string          `json:"Description"`
	InterfaceType string          `json:"InterfaceType"`
	Name          string          `json:"Name"`
	Model         string          `json:"Model"`
	Status        DiskDriveStatus `json:"Status"`
	// Check for logical drive, if disk drive, should return nothing.
	LogicalDriveName string `json:"LogicalDriveName,omitempty"`
}

// Disk Drive Status
type DiskDriveStatus struct {
	Health string `json:"Health"`
	State  string `json:"State"`
}

// Collection returns an array of the endpoints from the /ArrayControllers endpoint
type Collection struct {
	Members []struct {
		URL string `json:"@odata.id"`
	} `json:"Members"`
	MembersCount int `json:"Members@odata.count"`
}

// Main ArrayController JSON object

type ArrayControllerObject struct {
	Links Links  `json:"Links"`
	Model string `json:"Model"`
}

// Main ArrayController JSON object Links
type Links struct {
	LogicalDrives struct {
		URL string `json:"@odata.id"`
	}
	PhysicalDrives struct {
		URL string `json:"@odata.id"`
	}
}
