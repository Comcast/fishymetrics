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

type LinksWrapper struct {
	LinksURLSlice []string
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

func (w *LinksWrapper) UnmarshalJSON(data []byte) error {
	// Because of a change in output between firmware versions, we need to account for this.
	// Try to unmarshal as a slice of structs:
	// [
	//   {
	//     "@odata.id": "/redfish/v1/Systems/XXXX"
	//   }
	// ]
	if err := w.UnmarshalLinks(data); err == nil {
		return nil
	}

	// Next, try to unmarshal as a single Link object.
	// {
	//	"@odata.id":"/redfish/v1/Systems/XXXX"
	// }

	if err := w.UnmarshalObject(data); err == nil {
		return nil
	}

	// Fallback to unmarshal as a slice of strings
	// [
	//   "/redfish/v1/Systems/XXXX"
	// ]
	return json.Unmarshal(data, &w.LinksURLSlice)
}

func (w *LinksWrapper) UnmarshalLinks(data []byte) error {
	var links []struct {
		URL  string `json:"@odata.id,omitempty"`
		HRef string `json:"href,omitempty"`
	}
	if err := json.Unmarshal(data, &links); err != nil {
		return err
	}
	for _, link := range links {
		w.appendLink(link.URL, link.HRef)
	}
	return nil
}

func (w *LinksWrapper) UnmarshalObject(data []byte) error {
	var link struct {
		URL  string `json:"@odata.id,omitempty"`
		HRef string `json:"href,omitempty"`
	}
	if err := json.Unmarshal(data, &link); err != nil {
		return err
	}
	w.appendLink(link.URL, link.HRef)
	return nil
}

func (w *LinksWrapper) appendLink(url, href string) {
	if url != "" {
		w.LinksURLSlice = append(w.LinksURLSlice, url)
	} else if href != "" {
		w.LinksURLSlice = append(w.LinksURLSlice, href)
	}
}
