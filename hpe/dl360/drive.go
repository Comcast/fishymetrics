/*
 * Copyright 2023 Comcast Cable Communications Management, LLC
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

package dl360

// /redfish/v1/Systems/1/SmartStorage/ArrayControllers/0/LogicalDrives/1

// DriveMetrics is the top level json object for DL360 Drive metadata
type DriveMetrics struct {
	ID                 string `json:"Id"`
	CapacityMiB        int    `json:"CapacityMiB"`
	Description        string `json:"Description"`
	InterfaceType      string `json:"InterfaceType"`
	LogicalDriveName   string `json:"LogicalDriveName"`
	LogicalDriveNumber int    `json:"LogicalDriveNumber"`
	Name               string `json:"Name"`
	Raid               string `json:"Raid"`
	Status             Status `json:"Status"`
	StripeSizeBytes    int    `json:"StripeSizeBytes"`
}
