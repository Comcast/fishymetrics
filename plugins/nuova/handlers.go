/*
 * Copyright 2025 Comcast Cable Communications Management, LLC
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

package nuova

import (
	"encoding/xml"
	"fmt"
)

// exportXMLDriveMetrics collects the drive metrics from the /nuova endpoint in xml format
// and sets the prometheus gauges
func (n *NuovaPlugin) exportXMLDriveMetrics(body []byte) error {

	var state float64
	var dm XMLDriveMetrics
	var drv = (*n.DeviceMetrics)["diskDriveMetrics"]
	err := xml.Unmarshal(body, &dm)
	if err != nil {
		return fmt.Errorf("error Unmarshalling XMLDriveMetrics - %v", err)
	}

	for _, drive := range dm.OutConfigs.Drives {
		if drive.Presence == "equipped" {
			if drive.Operability == "operable" {
				state = OK
			} else {
				state = BAD
			}
			(*drv)["driveStatus"].WithLabelValues(drive.Name, n.ChassisSerialNumber, n.Model, drive.Id, "", "", "", "").Set(state)
		}
	}

	return nil
}
