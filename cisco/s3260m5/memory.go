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

package s3260m5

// /redfish/v1/Systems/XXXXX/Memory/DIMM_X1

// MemoryMetrics is the top level json object for UCS S3260 M5 Memory metadata
type MemoryMetrics struct {
	Name             string `json:"Name"`
	CapacityMiB      int    `json:"CapacityMiB"`
	Manufacturer     string `json:"Manufacturer"`
	MemoryDeviceType string `json:"MemoryDeviceType"`
	PartNumber       string `json:"PartNumber"`
	SerialNumber     string `json:"SerialNumber"`
	Status           Status `json:"Status"`
}
