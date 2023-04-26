package dl20

// /redfish/v1/Chassis/1/Thermal/

// ThermalMetrics is the top level json object for DL20 Thermal metadata
type ThermalMetrics struct {
	ID           string        `json:"Id"`
	Fans         []Fan         `json:"Fans"`
	Name         string        `json:"Name"`
	Temperatures []Temperature `json:"Temperatures"`
}

// Fan is the json object for a DL20 fan module
type Fan struct {
	MemberID     string `json:"MemberId"`
	Name         string `json:"Name"`
	Reading      int    `json:"Reading"`
	ReadingUnits string `json:"ReadingUnits"`
	Status       Status `json:"Status"`
}

// Temperature is the json object for a DL20 temperature sensor module
type Temperature struct {
	MemberID               string `json:"MemberId"`
	Name                   string `json:"Name"`
	PhysicalContext        string `json:"PhysicalContext"`
	ReadingCelsius         int    `json:"ReadingCelsius"`
	SensorNumber           int    `json:"SensorNumber"`
	Status                 Status `json:"Status"`
	UpperThresholdCritical int    `json:"UpperThresholdCritical"`
	UpperThresholdFatal    int    `json:"UpperThresholdFatal"`
}
