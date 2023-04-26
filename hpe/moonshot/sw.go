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

package moonshot

// /rest/v1/chassis/1/switches/sa

// Sw is the top level json object for Switch Information metadata
type Sw struct {
	Name         string   `json:"Name"`
	Power        string   `json:"Power,omitempty"`
	SerialNumber string   `json:"SerialNumber"`
	Status       SwStatus `json:"Status"`
	SwitchInfo   SwInfo   `json:"SwitchInfo"`
	Type         string   `json:"Type"`
}

// SwStatus is the top level json object for switch status
type SwStatus struct {
	State string `json:"State,omitempty"`
}

// SwInfo is the top level json object for switch info
type SwInfo struct {
	HealthStatus string `json:"HealthStatus,omitempty"`
}
