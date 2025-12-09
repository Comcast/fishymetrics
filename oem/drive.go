/*
 * Copyright 2025 Comcast Cable Communications Management, LLC
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
	"strconv"
)

// /redfish/v1/Systems/X/SmartStorage/ArrayControllers/

type DriveProtocol struct {
	Protocol string `json:"Protocol"`
}

// NVME's
// /redfish/v1/chassis/X/
type NVMeDriveMetrics struct {
	ID               string                `json:"Id"`
	Model            string                `json:"Model"`
	Name             string                `json:"Name"`
	MediaType        string                `json:"MediaType"`
	Oem              Oem                   `json:"Oem"`
	PhysicalLocation PhysicalLocation      `json:"PhysicalLocation"`
	Protocol         string                `json:"Protocol"`
	Status           Status                `json:"Status"`
	FailurePredicted FailurePredictWrapper `json:"FailurePredicted"`
	CapacityBytes    CapacityBytesWrapper  `json:"CapacityBytes"`
	SerialNumber     string                `json:"SerialNumber"`
}

// Logical Drives
// /redfish/v1/Systems/X/SmartStorage/ArrayControllers/X/LogicalDrives/X/
type LogicalDriveMetrics struct {
	Id                     string               `json:"Id"`
	CapacityMiB            int                  `json:"CapacityMiB"`
	CapacityBytes          CapacityBytesWrapper `json:"CapacityBytes"`
	Description            string               `json:"Description"`
	DisplayName            string               `json:"DisplayName"`
	InterfaceType          string               `json:"InterfaceType"`
	Identifiers            []Identifiers        `json:"Identifiers"`
	Links                  DriveCollection      `json:"Links"`
	LogicalDriveName       string               `json:"LogicalDriveName"`
	LogicalDriveNumber     int                  `json:"LogicalDriveNumber"`
	Name                   string               `json:"Name"`
	Raid                   string               `json:"Raid"`
	RaidType               string               `json:"RAIDType"`
	Status                 Status               `json:"Status"`
	StripeSizebytes        int                  `json:"StripeSizebytes"`
	VolumeUniqueIdentifier string               `json:"VolumeUniqueIdentifier"`
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
	Id               string                `json:"Id"`
	CapacityMiB      int                   `json:"CapacityMiB"`
	CapacityBytes    CapacityBytesWrapper  `json:"CapacityBytes"`
	Description      string                `json:"Description"`
	InterfaceType    string                `json:"InterfaceType"`
	Name             string                `json:"Name"`
	Model            string                `json:"Model"`
	Status           Status                `json:"Status"`
	LocationWrap     LocationWrapper       `json:"Location"`
	PhysicalLocation PhysicalLocation      `json:"PhysicalLocation"`
	FailurePredicted FailurePredictWrapper `json:"FailurePredicted"`
	SerialNumber     string                `json:"SerialNumber"`
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

// FailurePredict handles both string and bool values for FailurePredicted field
type FailurePredictWrapper struct {
	Value bool
}

func (f *FailurePredictWrapper) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as bool first
	var boolVal bool
	err := json.Unmarshal(data, &boolVal)
	if err == nil {
		f.Value = boolVal
		return nil
	}

	// Try to unmarshal as string
	var stringVal string
	err = json.Unmarshal(data, &stringVal)
	if err == nil {
		// Convert string to bool
		switch stringVal {
		case "true", "True", "TRUE", "1", "yes", "Yes", "YES":
			f.Value = true
		case "false", "False", "FALSE", "0", "no", "No", "NO", "":
			f.Value = false
		default:
			f.Value = false // default to false for unknown string values
		}
		return nil
	}

	return err
}

// CapacityBytesWrapper handles both string and int values for CapacityBytes field
type CapacityBytesWrapper struct {
	Value int
}

func (c *CapacityBytesWrapper) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as int first
	var intVal int
	err := json.Unmarshal(data, &intVal)
	if err == nil {
		c.Value = intVal
		return nil
	}

	// Try to unmarshal as string
	var stringVal string
	err = json.Unmarshal(data, &stringVal)
	if err == nil {
		// Convert string to int
		if stringVal == "" {
			c.Value = 0
			return nil
		}

		// Use strconv.Atoi to convert string to int
		intVal, err := strconv.Atoi(stringVal)
		if err != nil {
			// If conversion fails, default to 0
			c.Value = 0
		} else {
			c.Value = intVal
		}
		return nil
	}

	return err
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
