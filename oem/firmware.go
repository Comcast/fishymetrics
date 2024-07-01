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

import "encoding/json"

// /redfish/v1/UpdateService/FirmwareInventory/XXXX/
type GenericFirmware struct {
	Id            string `json:"Id,omitempty"`
	Name          string `json:"Name"`
	Version       string `json:"Version,omitempty"`
	VersionString string `json:"VersionString,omitempty"`
	Location      string `json:"Location,omitempty"`
	Description   string `json:"Description,omitempty"`
	Status        Status `json:"Status,omitempty"`
}

// /redfish/v1/Systems/1/FirmwareInventory/
type ILO4Firmware struct {
	Current FirmwareWrapper `json:"Current"`
}

type FirmwareWrapper struct {
	FirmwareSlice
}

type FirmwareSlice struct {
	Firmware []GenericFirmware
}

// Function to unmarshal the FirmwareWrapper struct
func (w *FirmwareWrapper) UnmarshalJSON(data []byte) error {
	var fw GenericFirmware
	var jsonData map[string]interface{}
	err := json.Unmarshal(data, &jsonData)

	if err == nil {
		for _, items := range jsonData {
			for _, item := range items.([]interface{}) {
				component, _ := json.Marshal(item)
				err = json.Unmarshal(component, &fw)
				if err == nil {
					w.Firmware = append(w.Firmware, fw)
				}
			}
		}
	} else {
		return json.Unmarshal(data, &w.Firmware)
	}

	return nil
}
