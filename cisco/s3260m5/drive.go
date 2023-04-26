package s3260m5

// /redfish/v1/Systems/XXXXX/Storage/SBMezzX

type StorageControllerMetrics struct {
	Name               string              `json:"Name,omitempty"`
	StorageControllers []StorageController `json:"StorageControllers"`
	Drives             []struct {
		URL string `json:"@odata.id"`
	} `json:"Drives"`
}

// StorageController contains storage controller status metadata
type StorageController struct {
	Name            string `json:"Name"`
	Status          Status `json:"Status"`
	Manufacturer    string `json:"Manufacturer"`
	FirmwareVersion string `json:"FirmwareVersion"`
}

// Drive contains disk status metadata
type Drive struct {
	Name             string `json:"Name"`
	SerialNumber     string `json:"SerialNumber"`
	Protocol         string `json:"Protocol"`
	MediaType        string `json:"MediaType"`
	Status           Status `json:"Status"`
	CapableSpeedGbs  int    `json:"CapableSpeedGbs"`
	FailurePredicted bool   `json:"FailurePredicted"`
	CapacityBytes    int    `json:"CapacityBytes"`
}

// /redfish/v1/Systems/XXXXX/SimpleStorage/SBMezzX

// DrivesMetrics contains drive status information all in one API call
type DriveMetrics struct {
	Devices []DriveStatus `json:"Devices"`
}

type DriveStatus struct {
	Name          string `json:"Name"`
	Status        Status `json:"Status"`
	CapacityBytes int    `json:"CapacityBytes,omitempty"`
	Manufacturer  string `json:"Manufacturer,omitempty"`
}
