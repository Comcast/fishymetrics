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

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
)

// /redfish/v1/Systems/WZPXXXXX/Storage/MRAID

type StorageControllerMetrics struct {
	Name              string                   `json:"Name"`
	StorageController StorageControllerWrapper `json:"StorageControllers"`
	Drives            []Drive                  `json:"Drives"`
}

// StorageController contains status metadata of the C220 chassis storage controller
type StorageController struct {
	Status          Status `json:"Status"`
	MemberId        string `json:"MemberId"`
	Model           string `json:"Model"`
	Name            string `json:"Name"`
	FirmwareVersion string `json:"FirmwareVersion"`
}

type StorageControllerSlice struct {
	StorageController []StorageController
}

type StorageControllerWrapper struct {
	StorageControllerSlice
}

func (w *StorageControllerWrapper) UnmarshalJSON(data []byte) error {
	// because of a change in output betwen c220 firmware versions we need to account for this
	if bytes.Compare([]byte("{"), data[0:1]) == 0 {
		var storCtlSlice StorageController
		err := json.Unmarshal(data, &storCtlSlice)
		if err != nil {
			return err
		}
		s := make([]StorageController, 0)
		s = append(s, storCtlSlice)
		w.StorageController = s
		return nil
	}
	return json.Unmarshal(data, &w.StorageController)
}

type Drive struct {
	Url string `json:"@odata.id"`
}

type Error struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code         string         `json:"code"`
	Message      string         `json:"message"`
	ExtendedInfo []ExtendedInfo `json:"@Message.ExtendedInfo"`
}

type ExtendedInfo struct {
	OdataType  string   `json:"@odata.type"`
	MessageID  string   `json:"MessageId"`
	Message    string   `json:"Message"`
	MessageArg []string `json:"MessageArgs"`
	Severity   string   `json:"Severity"`
}

// /redfish/v1/Systems/WZPXXXXX/Storage/MRAID/Drives/X

type DriveMetrics struct {
	Id            string `json:"Id"`
	Name          string `json:"Name"`
	Model         string `json:"Model"`
	CapacityBytes int    `json:"CapacityBytes"`
	Status        Status `json:"Status"`
}

// XML class_id="StorageLocalDiskSlotEp"

type XMLDriveMetrics struct {
	XMLName    xml.Name   `xml:"configResolveClass"`
	OutConfigs OutConfigs `xml:"outConfigs"`
}

type OutConfigs struct {
	XMLName xml.Name                 `xml:"outConfigs"`
	Drives  []StorageLocalDiskSlotEp `xml:"storageLocalDiskSlotEp"`
}

type StorageLocalDiskSlotEp struct {
	Id          string `xml:"id,attr"`
	Name        string `xml:"dn,attr"`
	Operability string `xml:"operability,attr"`
	Presence    string `xml:"presence,attr"`
}
