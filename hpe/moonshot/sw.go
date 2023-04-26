package moonshot

// /rest/v1/chassis/1/switches/sa

// Sw is the top level json object for Switch Information metadata
type Sw struct {
	Name         string   `json:"Name"`
	Power        string   `json:"Power,omitempty"`
	SerialNumber string   `json:"SerialNumber"`
	Status       SwStatus `json:"Status"`
	SwitchInfo   SwInfo   `json:"SwitchInfo"`
	Type         string   `json:"Type"`
}

// SwStatus is the top level json object for switch status
type SwStatus struct {
	State string `json:"State,omitempty"`
}

// SwInfo is the top level json object for switch info
type SwInfo struct {
	HealthStatus string `json:"HealthStatus,omitempty"`
}
