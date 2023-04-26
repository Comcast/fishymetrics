package s3260m5

// /redfish/v1/Chassis/XXXX/Thermal/

// ThermalMetrics is the top level json object for UCS S3260 M5 Thermal metadata
type ThermalMetrics struct {
	Status       Status        `json:"Status"`
	Fans         []Fan         `json:"Fans,omitempty"`
	Name         string        `json:"Name"`
	Temperatures []Temperature `json:"Temperatures"`
	Url          string        `json:"@odata.id"`
}

// Fan is the json object for a UCS S3260 M5 fan module
type Fan struct {
	Name         string `json:"Name"`
	Reading      int    `json:"Reading"`
	ReadingUnits string `json:"ReadingUnits"`
	Status       Status `json:"Status"`
}

// Temperature is the json object for a UCS S3260 M5 temperature sensor module
type Temperature struct {
	Name                   string  `json:"Name"`
	PhysicalContext        string  `json:"PhysicalContext"`
	ReadingCelsius         float64 `json:"ReadingCelsius"`
	Status                 Status  `json:"Status"`
	UpperThresholdCritical int     `json:"UpperThresholdCritical"`
}
