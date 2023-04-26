package dl20

// /redfish/v1/systems/1/

// MemoryMetrics is the top level json object for DL20 Memory metadata
type MemoryMetrics struct {
	ID            string        `json:"Id"`
	MemorySummary MemorySummary `json:"MemorySummary"`
}

// MemorySummary is the json object for DL20 MemorySummary metadata
type MemorySummary struct {
	Status                         StatusMemory `json:"Status"`
	TotalSystemMemoryGiB           int          `json:"TotalSystemMemoryGiB"`
	TotalSystemPersistentMemoryGiB int          `json:"TotalSystemPersistentMemoryGiB"`
}

// StatusMemory is the variable to determine if the memory is OK or not
type StatusMemory struct {
	HealthRollup string `json:"HealthRollup"`
}
