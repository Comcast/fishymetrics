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

// /redfish/v1/UpdateService/FirmwareInventory/

package oem

// Collection returns an array of the endpoints from the FirmwareInventory
type FirmwareComponents struct {
	Members []struct {
		URL string `json:"@odata.id"`
	} `json:"Members"`
	MembersCount int `json:"Members@odata.count"`
}

// Collection returns the component details

// /redfish/v1/UpdateService/FirmwareInventory/XXXX/
type FirmwareComponent struct {
	Name        string `json:"Name"`
	Description string `json:"Description"`
	Version     string `json:"Version"`
}
