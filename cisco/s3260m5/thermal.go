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

// /redfish/v1/Chassis/XXXX/Thermal/

// ThermalMetrics is the top level json object for UCS S3260 M5 Thermal metadata
type ThermalMetrics struct {
	Status       Status        `json:"Status"`
	Fans         []Fan         `json:"Fans,omitempty"`
	Name         string        `json:"Name"`
	Temperatures []Temperature `json:"Temperatures"`
	Url          string        `json:"@odata.id"`
}

// Fan is the json object for a UCS S3260 M5 fan module
type Fan struct {
	Name         string `json:"Name"`
	Reading      int    `json:"Reading"`
	ReadingUnits string `json:"ReadingUnits"`
	Status       Status `json:"Status"`
}

// Temperature is the json object for a UCS S3260 M5 temperature sensor module
type Temperature struct {
	Name                   string  `json:"Name"`
	PhysicalContext        string  `json:"PhysicalContext"`
	ReadingCelsius         float64 `json:"ReadingCelsius"`
	Status                 Status  `json:"Status"`
	UpperThresholdCritical int     `json:"UpperThresholdCritical"`
}
