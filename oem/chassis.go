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

// /redfish/v1/Chassis/XXXXX
type Chassis struct {
	Links      ChassisLinks `json:"Links"`
	LinksLower ChassisLinks `json:"links"`
	PowerAlt   Link         `json:"Power"`
	ThermalAlt Link         `json:"Thermal"`
}

type ChassisLinks struct {
	System  LinksWrapper `json:"ComputerSystems"`
	Storage LinksWrapper `json:"Storage"`
	Drives  LinksWrapper `json:"Drives"`
	Power   LinksWrapper `json:"PoweredBy"`
	Thermal LinksWrapper `json:"CooledBy"`
}

type LinksURL struct {
	LinksURLSlice []string
}

type LinksWrapper struct {
	LinksURL
}

func (w *LinksWrapper) UnmarshalJSON(data []byte) error {
	// because of a change in output between firmware versions we need to account for this
	if bytes.Compare([]byte("[{"), data[0:2]) == 0 {
		var linksTmp []struct {
			URL  string `json:"@odata.id,omitempty"`
			HRef string `json:"href,omitempty"`
		}
		err := json.Unmarshal(data, &linksTmp)
		if len(linksTmp) > 0 {
			for _, l := range linksTmp {
				if l.URL != "" {
					w.LinksURLSlice = append(w.LinksURLSlice, l.URL)
				} else {
					w.LinksURLSlice = append(w.LinksURLSlice, l.HRef)
				}
			}
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &w.LinksURLSlice)
}

type ChassisStorageBattery struct {
	Oem OemSys `json:"Oem"`
}

type HRef struct {
	URL string `json:"href"`
}

type Link struct {
	URL string `json:"@odata.id"`
}
