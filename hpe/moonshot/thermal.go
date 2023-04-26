package moonshot

// /rest/v1/Chassis/1/ThermalMetrics

// HpThermal is hold the metadata for product information
type HpThermal struct {
	Location string `json:"Location"`
	Type     string `json:"Type"`
}

// OemThermal is the top level json object for Hp product information
type OemThermal struct {
	Hp HpThermal `json:"Hp"`
}

// StatusThermal is the variable to determine if a Fan is OK or not
type StatusThermal struct {
	State string `json:"State"`
}

// SamplesThermal saves historical data on specific intervals
type SamplesThermal struct {
	Temperature int `json:"Temperature"`
}

// Fans gives metadata on each fan
type Fans struct {
	CurrentReading int           `json:"CurrentReading"`
	FanName        string        `json:"FanName"`
	Oem            OemThermal    `json:"Oem"`
	ProductName    string        `json:"ProductName"`
	Status         StatusThermal `json:"Status"`
	Units          string        `json:"Units"`
}

// ThermalMetrics is the top level json object for Fan and Temperatures metadata
type ThermalMetrics struct {
	Fans         []Fans         `json:"Fans"`
	Name         string         `json:"Name"`
	Temperatures []Temperatures `json:"Temperatures"`
	Type         string         `json:"Type"`
}

// Temperatures is the top level json object for temperature metadata
type Temperatures struct {
	CurrentReading            int                       `json:"CurrentReading"`
	Name                      string                    `json:"Name"`
	Status                    StatusThermal             `json:"Status"`
	TemperatureHistoryLevel   []TemperatureHistoryLevel `json:"TemperatureHistoryLevel"`
	Units                     string                    `json:"Units"`
	UpperThresholdCritical    int                       `json:"UpperThresholdCritical,omitempty"`
	UpperThresholdNonCritical int                       `json:"UpperThresholdNonCritical,omitempty"`
}

// TemperatureHistoryLevel is the top level json object for all historical Samples metadata
type TemperatureHistoryLevel struct {
	Counter    int              `json:"Counter"`
	Cumulator  int              `json:"Cumulator"`
	SampleType string           `json:"SampleType"`
	Samples    []SamplesThermal `json:"Samples"`
}
