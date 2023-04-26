package s3260m4

import (
	"bytes"
	"encoding/json"
)

// /redfish/v1/Managers/BMC2

// Chassis contains the Model Number, Firmware, etc of the chassis
type Chassis struct {
	FirmwareVersion string `json:"FirmwareVersion"`
	Links           struct {
		ManagerForServers ServerManagerURLWrapper `json:"ManagerForServers"`
	} `json:"Links"`
	Model       string `json:"Model"`
	Description string `json:"Description"`
}

type ServerManagerURL struct {
	ServerManagerURLSlice []string
}

type ServerManagerURLWrapper struct {
	ServerManagerURL
}

func (w *ServerManagerURLWrapper) UnmarshalJSON(data []byte) error {
	// because of a change in output betwen s3260m4 firmware versions we need to account for this
	if bytes.Compare([]byte("[{"), data[0:2]) == 0 {
		var serMgrTmp []struct {
			Url string `json:"@odata.id,omitempty"`
		}
		err := json.Unmarshal(data, &serMgrTmp)
		if len(serMgrTmp) > 0 {
			s := make([]string, 0)
			s = append(s, serMgrTmp[0].Url)
			w.ServerManagerURLSlice = s
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &w.ServerManagerURLSlice)
}

// Collection returns an array of the endpoints from the chassis pertaining to a resource type
type Collection struct {
	Members []struct {
		URL string `json:"@odata.id"`
	} `json:"Members"`
	MembersCount int `json:"Members@odata.count"`
}

// Status contains metadata for the health of a particular component/module
type Status struct {
	Health       string `json:"Health,omitempty"`
	HealthRollup string `json:"HealthRollup,omitempty"`
	State        string `json:"State,omitempty"`
}

// /redfish/v1/Systems/XXXXX

// ServerManager contains the BIOS version of the chassis
type ServerManager struct {
	BiosVersion string `json:"BiosVersion"`
}

// /redfish/v1/Chassis/CMC

// Chassis contains the Model Number, Firmware, etc of the chassis
type ChassisSerialNumber struct {
	SerialNumber string `json:"SerialNumber"`
}
