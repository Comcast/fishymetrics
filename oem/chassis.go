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
type Chassis struct {
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

// Collection returns an array of the endpoints from the chassis pertaining to a resource type
type Collection struct {
	Members []struct {
		URL string `json:"@odata.id"`
	} `json:"Members"`
	MembersCount int `json:"Members@odata.count"`
}

// Status contains metadata for the health of a particular component/module
type Status struct {
	Health       string `json:"Health,omitempty"`
	HealthRollup string `json:"HealthRollup,omitempty"`
	State        string `json:"State,omitempty"`
}

// /redfish/v1/Systems/XXXXX

// ServerManager contains the BIOS version and Serial number of the chassis
type ServerManager struct {
	BiosVersion   string `json:"BiosVersion"`
	SerialNumber  string `json:"SerialNumber"`
	IloServerName string `json:"HostName"`
}

// /redfish/v1/Chassis/CMC

// Chassis contains the Model Number, Firmware, etc of the chassis
type ChassisSerialNumber struct {
	SerialNumber string `json:"SerialNumber"`
}

// /redfish/v1/Systems/1/ or /redfish/v1/Managers/1/
type SystemMetrics struct {
	Oem OemSys `json:"Oem"`
}

type OemSys struct {
	Hpe HpeSys `json:"Hpe,omitempty"`
	Hp  HpeSys `json:"Hp,omitempty"`
}

type HpeSys struct {
	Battery     []StorageBattery `json:"Battery"`
	IloSelfTest []IloSelfTest    `json:"iLOSelfTestResults"`
}

type StorageBattery struct {
	Condition    string `json:"Condition"`
	Index        int    `json:"Index"`
	Model        string `json:"Model"`
	Present      string `json:"Present"`
	Name         string `json:"ProductName"`
	SerialNumber string `json:"SerialNumber"`
}

type IloSelfTest struct {
	Name   string `json:"SelfTestName"`
	Status string `json:"Status"`
	Notes  string `json:"Notes"`
}
