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

package c220

// /redfish/v1/Systems/XXXXX/Processors/CPUX

// ProcessorMetrics is the top level json object for UCS C220 Processor metadata
type ProcessorMetrics struct {
	Name                  string      `json:"Name"`
	Description           string      `json:"Description"`
	Status                Status      `json:"Status"`
	ProcessorArchitecture string      `json:"ProcessorArchitecture"`
	TotalThreads          interface{} `json:"TotalThreads"`
	TotalCores            interface{} `json:"TotalCores"`
	Model                 string      `json:"Model"`
}
