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

// /redfish/v1/Systems/WZPXXXXX/Storage/MRAID

type StorageControllerMetrics struct {
	ControllerFirmware FirmwareVersionWrapper   `json:"FirmwareVersion"`
	Drives             []Drive                  `json:"Drives"`
	Location           StorCtrlLocationWrapper  `json:"Location"`
	Model              string                   `json:"Model"`
	Name               string                   `json:"Name"`
	StorageController  StorageControllerWrapper `json:"StorageControllers"`
	Status             Status                   `json:"Status"`
}

// StorageController contains status metadata of the C220 chassis storage controller
type StorageController struct {
	Status          Status                  `json:"Status"`
	MemberId        string                  `json:"MemberId"`
	Manufacturer    string                  `json:"Manufacturer,omitempty"`
	Model           string                  `json:"Model"`
	Name            string                  `json:"Name"`
	FirmwareVersion string                  `json:"FirmwareVersion"`
	Location        StorCtrlLocationWrapper `json:"Location,omitempty"`
}

type StorageControllerSlice struct {
	StorageController []StorageController
}

type StorageControllerWrapper struct {
	StorageControllerSlice
}

func (w *StorageControllerWrapper) UnmarshalJSON(data []byte) error {
	// because of a change in output between firmware versions we need to account for this
	// StorageController can either be []StorageController or a singular StorageController
	var storCtl StorageController
	err := json.Unmarshal(data, &storCtl)
	if err == nil {
		sc := make([]StorageController, 0)
		sc = append(sc, storCtl)
		w.StorageController = sc
	} else {
		return json.Unmarshal(data, &w.StorageController)
	}

	return nil
}

type FirmwareVersionWrapper struct {
	FirmwareVersion string
}

func (w *FirmwareVersionWrapper) UnmarshalJSON(data []byte) error {
	// because of a change in output between firmware versions we need to account for this
	// firmware version can either be a string or a json object
	var CurrentFirmware struct {
		Current struct {
			VersionString string `json:"VersionString"`
		} `json:"Current"`
	}
	err := json.Unmarshal(data, &CurrentFirmware)
	if err == nil {
		w.FirmwareVersion = CurrentFirmware.Current.VersionString
	} else {
		return json.Unmarshal(data, &w.FirmwareVersion)
	}

	return nil
}

type Drive struct {
	Url string `json:"@odata.id"`
}

type Error struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code         string         `json:"code"`
	Message      string         `json:"message"`
	ExtendedInfo []ExtendedInfo `json:"@Message.ExtendedInfo"`
}

type ExtendedInfo struct {
	OdataType  string   `json:"@odata.type"`
	MessageID  string   `json:"MessageId"`
	Message    string   `json:"Message"`
	MessageArg []string `json:"MessageArgs"`
	Severity   string   `json:"Severity"`
}

type StorCtrlLocationWrapper struct {
	Location string
}

func (w *StorCtrlLocationWrapper) UnmarshalJSON(data []byte) error {
	var location PhysicalLocation
	err := json.Unmarshal(data, &location)
	if err == nil {
		w.Location = location.PartLocation.ServiceLabel
	} else {
		return json.Unmarshal(data, &w.Location)
	}

	return nil
}
