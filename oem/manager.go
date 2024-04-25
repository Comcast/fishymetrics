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

// /redfish/v1/Managers/XX/

// Chassis contains the Model Number, Firmware, etc of the chassis
type Manager struct {
	FirmwareVersion string `json:"FirmwareVersion"`
	LinksUpper      struct {
		ManagerForServers ServerManagerURLWrapper `json:"ManagerForServers"`
	} `json:"Links"`
	LinksLower struct {
		ManagerForServers ServerManagerURLWrapper `json:"ManagerForServers"`
	} `json:"links"`
	Model       string `json:"Model"`
	Description string `json:"Description"`
}

type ServerManagerURL struct {
	ServerManagerURLSlice []string
}

type ServerManagerURLWrapper struct {
	ServerManagerURL
}

func (w *ServerManagerURLWrapper) UnmarshalJSON(data []byte) error {
	// because of a change in output between firmware versions we need to account for this
	// try to unmarshal as a slice of structs
	// [
	//   {
	//     "@odata.id": "/redfish/v1/Systems/XXXX"
	//   }
	// ]
	var svrMgr []struct {
		URL  string `json:"@odata.id,omitempty"`
		HRef string `json:"href,omitempty"`
	}
	err := json.Unmarshal(data, &svrMgr)
	if err == nil {
		if len(svrMgr) > 0 {
			for _, l := range svrMgr {
				if l.URL != "" {
					w.ServerManagerURLSlice = append(w.ServerManagerURLSlice, l.URL)
				} else {
					w.ServerManagerURLSlice = append(w.ServerManagerURLSlice, l.HRef)
				}
			}
		}
	} else {
		// try to unmarshal as a slice of strings
		// [
		//   "/redfish/v1/Systems/XXXX"
		// ]
		return json.Unmarshal(data, &w.ServerManagerURLSlice)
	}

	return nil
}

type IloSelfTest struct {
	Name   string `json:"SelfTestName"`
	Status string `json:"Status"`
	Notes  string `json:"Notes"`
}
