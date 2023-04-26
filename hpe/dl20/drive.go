package dl20

// /redfish/v1/Systems/1/SmartStorage/ArrayControllers/0/LogicalDrives/1

// DriveMetrics is the top level json object for DL20 Drive metadata
type DriveMetrics struct {
	ID                 string `json:"Id"`
	CapacityMiB        int    `json:"CapacityMiB"`
	Description        string `json:"Description"`
	InterfaceType      string `json:"InterfaceType"`
	LogicalDriveName   string `json:"LogicalDriveName"`
	LogicalDriveNumber int    `json:"LogicalDriveNumber"`
	Name               string `json:"Name"`
	Raid               string `json:"Raid"`
	Status             Status `json:"Status"`
	StripeSizeBytes    int    `json:"StripeSizeBytes"`
}
