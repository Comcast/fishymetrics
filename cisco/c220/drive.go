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
	Drives            []struct {
		Url string `json:"@odata.id"`
	} `json:"Drives"`
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
