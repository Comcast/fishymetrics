package s3260m4

// /redfish/v1/Systems/XXXXX/Memory/DIMM_X1

// MemoryMetrics is the top level json object for UCS S3260 M4 Memory metadata
type MemoryMetrics struct {
	Name             string      `json:"Name"`
	CapacityMiB      interface{} `json:"CapacityMiB"`
	Manufacturer     string      `json:"Manufacturer"`
	MemoryDeviceType string      `json:"MemoryDeviceType"`
	PartNumber       string      `json:"PartNumber"`
	SerialNumber     string      `json:"SerialNumber"`
	Status           interface{} `json:"Status"`
}
