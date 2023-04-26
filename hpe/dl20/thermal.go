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

package dl20

// /redfish/v1/Chassis/1/Thermal/

// ThermalMetrics is the top level json object for DL20 Thermal metadata
type ThermalMetrics struct {
	ID           string        `json:"Id"`
	Fans         []Fan         `json:"Fans"`
	Name         string        `json:"Name"`
	Temperatures []Temperature `json:"Temperatures"`
}

// Fan is the json object for a DL20 fan module
type Fan struct {
	MemberID     string `json:"MemberId"`
	Name         string `json:"Name"`
	Reading      int    `json:"Reading"`
	ReadingUnits string `json:"ReadingUnits"`
	Status       Status `json:"Status"`
}

// Temperature is the json object for a DL20 temperature sensor module
type Temperature struct {
	MemberID               string `json:"MemberId"`
	Name                   string `json:"Name"`
	PhysicalContext        string `json:"PhysicalContext"`
	ReadingCelsius         int    `json:"ReadingCelsius"`
	SensorNumber           int    `json:"SensorNumber"`
	Status                 Status `json:"Status"`
	UpperThresholdCritical int    `json:"UpperThresholdCritical"`
	UpperThresholdFatal    int    `json:"UpperThresholdFatal"`
}
