package s3260m5

// /redfish/v1/Systems/XXXXX/Processors/CPUX

// ProcessorMetrics is the top level json object for UCS C220 Processor metadata
type ProcessorMetrics struct {
	Name                  string `json:"Name"`
	Description           string `json:"Description"`
	Status                Status `json:"Status"`
	ProcessorArchitecture string `json:"ProcessorArchitecture"`
	TotalThreads          int    `json:"TotalThreads"`
	TotalCores            int    `json:"TotalCores"`
	Model                 string `json:"Model"`
}
