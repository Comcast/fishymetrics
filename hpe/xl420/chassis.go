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

package xl420

// Collection returns an array of the endpoints from the chassis pertaining to a resource type
type Collection struct {
	Members []struct {
		URL string `json:"@odata.id"`
	} `json:"Members"`
	MembersCount int `json:"Members@odata.count"`
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
