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
	"bytes"
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
	// because of a change in output betwen c220 firmware versions we need to account for this
	if bytes.Compare([]byte("[{"), data[0:2]) == 0 {
		var serMgrTmp []struct {
			UrlLinks string `json:"@odata.id,omitempty"`
			Urllinks string `json:"href,omitempty"`
		}
		err := json.Unmarshal(data, &serMgrTmp)
		if len(serMgrTmp) > 0 {
			s := make([]string, 0)
			if serMgrTmp[0].UrlLinks != "" {
				s = append(s, serMgrTmp[0].UrlLinks)
			} else {
				s = append(s, serMgrTmp[0].Urllinks)
			}
			w.ServerManagerURLSlice = s
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &w.ServerManagerURLSlice)
}

type IloSelfTest struct {
	Name   string `json:"SelfTestName"`
	Status string `json:"Status"`
	Notes  string `json:"Notes"`
}
