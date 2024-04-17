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
	Links ChassisEndpoints `json:"Links"`
}

type ChassisEndpoints struct {
	System  []Link `json:"ComputerSystems"`
	Storage []Link `json:"Storage"`
	Drives  []Link `json:"Drives"`
	Power   []Link `json:"PoweredBy"`
	Thermal []Link `json:"CooledBy"`
}

type Link struct {
	URL string `json:"@odata.id"`
}

type ChassisStorageBattery struct {
	Oem OemSys `json:"Oem"`
}
