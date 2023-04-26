package s3260m4

// /redfish/v1/Systems/XXXXX/Processors/CPUX

// ProcessorMetrics is the top level json object for UCS S3260 M4 Processor metadata
type ProcessorMetrics struct {
	Name                  string      `json:"Name"`
	Description           string      `json:"Description"`
	Status                Status      `json:"Status"`
	ProcessorArchitecture string      `json:"ProcessorArchitecture"`
	TotalThreads          interface{} `json:"TotalThreads"`
	TotalCores            interface{} `json:"TotalCores"`
	Model                 string      `json:"Model"`
}
