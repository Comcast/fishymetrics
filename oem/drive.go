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

import (
	"encoding/json"
)

// /redfish/v1/Systems/X/SmartStorage/ArrayControllers/

type DriveProtocol struct {
	Protocol string `json:"Protocol"`
}

// NVME's
// /redfish/v1/chassis/X/
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
// /redfish/v1/Systems/X/SmartStorage/ArrayControllers/X/LogicalDrives/X/
type LogicalDriveMetrics struct {
	Id                     string          `json:"Id"`
	CapacityMiB            int             `json:"CapacityMiB"`
	Description            string          `json:"Description"`
	DisplayName            string          `json:"DisplayName"`
	InterfaceType          string          `json:"InterfaceType"`
	Identifiers            []Identifiers   `json:"Identifiers"`
	Links                  DriveCollection `json:"Links"`
	LogicalDriveName       string          `json:"LogicalDriveName"`
	LogicalDriveNumber     int             `json:"LogicalDriveNumber"`
	Name                   string          `json:"Name"`
	Raid                   string          `json:"Raid"`
	RaidType               string          `json:"RAIDType"`
	Status                 Status          `json:"Status"`
	StripeSizebytes        int             `json:"StripeSizebytes"`
	VolumeUniqueIdentifier string          `json:"VolumeUniqueIdentifier"`
}

type DriveCollection struct {
	DrivesCount int `json:"Drives@odata.count"`
}

type Identifiers struct {
	DurableName string `json:"DurableName"`
}

// Disk Drives
// /redfish/v1/Systems/XXXXX/SmartStorage/ArrayControllers/X/DiskDrives/X/
// /redfish/v1/Systems/XXXXX/Storage/XXXXX/Drives/PD-XX/
type DiskDriveMetrics struct {
	Id               string           `json:"Id"`
	CapacityMiB      int              `json:"CapacityMiB"`
	CapacityBytes    int              `json:"CapacityBytes"`
	Description      string           `json:"Description"`
	InterfaceType    string           `json:"InterfaceType"`
	Name             string           `json:"Name"`
	Model            string           `json:"Model"`
	Status           Status           `json:"Status"`
	LocationWrap     LocationWrapper  `json:"Location"`
	PhysicalLocation PhysicalLocation `json:"PhysicalLocation"`
	SerialNumber     string           `json:"SerialNumber"`
}

type LocationWrapper struct {
	Location string
}

func (w *LocationWrapper) UnmarshalJSON(data []byte) error {
	// because of a change in output between firmware versions we need to account for this
	// try to unmarshal as a slice of structs
	// [
	//   {
	//     "Info": "location data"
	//   }
	// ]
	var loc []struct {
		Loc string `json:"Info,omitempty"`
	}
	err := json.Unmarshal(data, &loc)
	if err == nil {
		if len(loc) > 0 {
			for _, l := range loc {
				if l.Loc != "" {
					w.Location = l.Loc
				}
			}
		}
	} else {
		// try to unmarshal as a string
		// {
		//   ...
		//   "Location": "location data"
		//   ...
		// }
		return json.Unmarshal(data, &w.Location)
	}

	return nil

}

// GenericDrive is used to iterate over differing drive endpoints
// /redfish/v1/Systems/X/SmartStorage/ArrayControllers/ for Logical and Physical Drives
// /redfish/v1/Chassis/X/Drives/ for NVMe Drive(s)
type GenericDrive struct {
	Members       []Members    `json:"Members,omitempty"`
	LinksUpper    LinksUpper   `json:"Links,omitempty"`
	LinksLower    LinksLower   `json:"links,omitempty"`
	MembersCount  int          `json:"Members@odata.count,omitempty"`
	DriveCount    int          `json:"Drives@odata.count,omitempty"`
	StorageDrives []Link       `json:"Drives,omitempty"`
	Volumes       LinksWrapper `json:"Volumes,omitempty"`
	Controllers   Link         `json:"Controllers,omitempty"`
}

type Members struct {
	URL string `json:"@odata.id"`
}

type LinksUpper struct {
	Drives         []Link `json:"Drives,omitempty"`
	LogicalDrives  Link   `json:"LogicalDrives,omitempty"`
	PhysicalDrives Link   `json:"PhysicalDrives,omitempty"`
}

type LinksLower struct {
	Drives         []HRef `json:"Drives,omitempty"`
	LogicalDrives  HRef   `json:"LogicalDrives,omitempty"`
	PhysicalDrives HRef   `json:"PhysicalDrives,omitempty"`
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
