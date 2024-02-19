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

// Logical Drives
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

// GenericDrive is used to iterate over differing drive endpoints
type GenericDrive struct {
	Members []struct {
		URL string `json:"@odata.id"`
	} `json:"Members,omitempty"`
	Links struct {
		Drives []struct {
			URL string `json:"@odata.id"`
		} `json:"Drives,omitempty"`
		LogicalDrives struct {
			URL string `json:"@odata.id"`
		} `json:"LogicalDrives,omitempty"`
		PhysicalDrives struct {
			URL string `json:"@odata.id"`
		} `json:"PhysicalDrives,omitempty"`
	} `json:"Links,omitempty"`
	MembersCount int `json:"@odata.count,omitempty"`
}

// Disk Drives
type DiskDriveMetrics struct {
	Id            string          `json:"Id"`
	CapacityMiB   int             `json:"CapacityMiB"`
	Description   string          `json:"Description"`
	InterfaceType string          `json:"InterfaceType"`
	Name          string          `json:"Name"`
	Model         string          `json:"Model"`
	Status        DiskDriveStatus `json:"Status"`
}

// Disk Drive Status
type DiskDriveStatus struct {
	Health string `json:"Health"`
	State  string `json:"State"`
}

// // ArrayController: /redfish/v1/Systems/1/SmartStorage/ArrayControllers/
// type ArrayController struct {
// 	Members      Members `json:"Members"`
// 	MembersCount int     `json:"@odata.count"`
// }

// // ArrayController Members
// type Members struct {
// 	URL string `json:"@odata.id"`
// }

// type Controller struct {
// 	Links Links `json:"Links"`
// }

// // ArrayController Links
// type Links struct {
// 	LogicalDrives  driveURL `json:"LogicalDrives,omitempty"`
// 	PhysicalDrives driveURL `json:"PhysicalDrives,omitempty"` // aka DiskDrives
// 	Drives         driveURL `json:"Drives,omitempty"`         // used for chassis endpoint for NVME only
// }

// // URL string from within Logical Drives or Disk Drives
// type driveURL struct {
// 	URL string `json:"@odata.id"`
// }

// // GenericDrive is used to iterate over differing drive endpoints
// type GenericDrive struct {
// 	Members      Members `json:"Members,omitempty"`
// 	Links        Links   `json:"Links,omitempty"`
// 	MembersCount int     `json:"@odata.count,omitempty"`
// }
