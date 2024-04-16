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

package nuova

import (
	"github.com/comcast/fishymetrics/common"
	"github.com/comcast/fishymetrics/exporter"
	"github.com/comcast/fishymetrics/pool"
	"go.uber.org/zap"
)

const (
	// DRIVE_XML represents the logical drive metric endpoints using the XML endpoint
	DRIVE_XML = "storageLocalDiskSlotEp"
	// OK is a string representation of the float 1.0 for device status
	OK = 1.0
	// BAD is a string representation of the float 0.0 for device status
	BAD = 0.0
	// DISABLED is a string representation of the float -1.0 for device status
	DISABLED = -1.0
)

var (
	log *zap.Logger
)

type NuovaPlugin exporter.Exporter

func (n *NuovaPlugin) Apply(e *exporter.Exporter) error {

	var handlers []common.Handler

	log = zap.L()

	if e.GetPool() != nil {
		// check raid controller
		isPresent, err := checkRaidController(e.GetUrl()+"/redfish/v1/Systems/"+e.ChassisSerialNumber+"/Storage/MRAID", e.GetHost(), e.GetClient())
		if err != nil {
			log.Error("error when getting raid controller from "+e.Model, zap.Error(err), zap.Any("trace_id", e.GetContext().Value("traceID")))
			return err
		}
		if !isPresent {
			// servers without raid controller class_id="storageLocalDiskSlotEp"
			n.ChassisSerialNumber = e.ChassisSerialNumber
			n.DeviceMetrics = e.DeviceMetrics
			n.Model = e.Model
			handlers = append(handlers, n.exportXMLDriveMetrics)
			e.GetPool().AddTask(pool.NewTask(FetchXML(e.GetUrl()+"/nuova", DRIVE_XML, e.GetHost(), e.GetClient()), handlers))
		}
	}
	return nil
}
