package dl360

// /redfish/v1/Chassis/1/Thermal/

// ThermalMetrics is the top level json object for DL360 Thermal metadata
type ThermalMetrics struct {
	ID           string        `json:"Id"`
	Fans         []Fan         `json:"Fans"`
	Name         string        `json:"Name"`
	Temperatures []Temperature `json:"Temperatures"`
}

// Fan is the json object for a DL360 fan module
type Fan struct {
	MemberID     string        `json:"MemberId"`
	Name         string        `json:"Name"`
	Reading      int           `json:"Reading"`
	ReadingUnits string        `json:"ReadingUnits"`
	Status       StatusThermal `json:"Status"`
}

// StatusThermal is the variable to determine if a fan or temperature sensor module is OK or not
type StatusThermal struct {
	Health string `json:"Health"`
	State  string `json:"State"`
}

// Temperature is the json object for a DL360 temperature sensor module
type Temperature struct {
	MemberID               string        `json:"MemberId"`
	Name                   string        `json:"Name"`
	PhysicalContext        string        `json:"PhysicalContext"`
	ReadingCelsius         int           `json:"ReadingCelsius"`
	SensorNumber           int           `json:"SensorNumber"`
	Status                 StatusThermal `json:"Status"`
	UpperThresholdCritical int           `json:"UpperThresholdCritical"`
	UpperThresholdFatal    int           `json:"UpperThresholdFatal"`
}
