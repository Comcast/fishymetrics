package c220

// /redfish/v1/Chassis/1/Thermal/

// ThermalMetrics is the top level json object for UCS C220 Thermal metadata
type ThermalMetrics struct {
	Status       Status        `json:"Status"`
	Fans         []Fan         `json:"Fans"`
	Name         string        `json:"Name"`
	Temperatures []Temperature `json:"Temperatures"`
	Url          string        `json:"@odata.id"`
}

// Fan is the json object for a UCS C220 fan module
type Fan struct {
	Name         string      `json:"Name"`
	Reading      interface{} `json:"Reading"`
	ReadingUnits string      `json:"ReadingUnits"`
	Status       Status      `json:"Status"`
}

// Temperature is the json object for a UCS C220 temperature sensor module
type Temperature struct {
	Name                   string      `json:"Name"`
	PhysicalContext        string      `json:"PhysicalContext"`
	ReadingCelsius         interface{} `json:"ReadingCelsius"`
	Status                 Status      `json:"Status"`
	UpperThresholdCritical interface{} `json:"UpperThresholdCritical"`
}
