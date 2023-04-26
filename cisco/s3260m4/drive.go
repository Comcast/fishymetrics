package s3260m4

import (
	"bytes"
	"encoding/json"
)

// /redfish/v1/Systems/XXXXX/Storage/SBMezzX

type StorageControllerMetrics struct {
	Name              string                   `json:"Name,omitempty"`
	StorageController StorageControllerWrapper `json:"StorageControllers"`
	Drives            []struct {
		URL string `json:"@odata.id"`
	} `json:"Drives"`
	Url string `json:"@odata.id"`
}

// StorageController contains storage controller status metadata
type StorageController struct {
	Name            string `json:"Name"`
	Status          Status `json:"Status"`
	Manufacturer    string `json:"Manufacturer"`
	FirmwareVersion string `json:"FirmwareVersion"`
}

type StorageControllerSlice struct {
	StorageController []StorageController
}

type StorageControllerWrapper struct {
	StorageControllerSlice
}

func (w *StorageControllerWrapper) UnmarshalJSON(data []byte) error {
	// because of a change in output betwen s3260m4 firmware versions we need to account for this
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

// Drive contains disk status metadata
type Drive struct {
	Name             string `json:"Name"`
	SerialNumber     string `json:"SerialNumber"`
	Protocol         string `json:"Protocol"`
	MediaType        string `json:"MediaType"`
	Status           Status `json:"Status"`
	CapableSpeedGbs  string `json:"CapableSpeedGbs"`
	FailurePredicted bool   `json:"FailurePredicted"`
	CapacityBytes    string `json:"CapacityBytes"`
}

// /redfish/v1/Systems/XXXXX/SimpleStorage/SBMezzX
// /redfish/v1/Systems/XXXXX/SimpleStorage/IOEMezz1

// DriveMetrics contains drive status information all in one API call
type DriveMetrics struct {
	Devices []DriveStatus `json:"Devices"`
}

type DriveStatus struct {
	Name          string `json:"Name"`
	Status        Status `json:"Status"`
	CapacityBytes int    `json:"CapacityBytes,omitempty"`
	Manufacturer  string `json:"Manufacturer,omitempty"`
}
