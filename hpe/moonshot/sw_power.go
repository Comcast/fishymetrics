package moonshot

// /rest/v1/chassis/1/switches/sa/PowerMetrics

// SwPowerMetrics is the top level json object for Power metadata
type SwPowerMetrics struct {
	Name          string          `json:"Name"`
	Oem           SwOemPower      `json:"Oem"`
	PowerSupplies []PowerSupplies `json:"PowerSupplies"`
	Type          string          `json:"Type"`
	Links         Links           `json:"links"`
}

// SwOemPower is the top level json object for historical data for wattage
type SwOemPower struct {
	Hp SwHpPower `json:"Hp"`
}

// SwHpPower is the top level json object for the power supplies metadata
type SwHpPower struct {
	InstantWattage      int                     `json:"InstantWattage"`
	MaximumWattage      int                     `json:"MaximumWattage"`
	Type                string                  `json:"Type"`
	WattageHistoryLevel []SwWattageHistoryLevel `json:"WattageHistoryLevel"`
}

// SwWattageHistoryLevel is the top level json object for all historical Samples metadata
type SwWattageHistoryLevel struct {
	Counter    int              `json:"Counter"`
	Cumulator  int              `json:"Cumulator"`
	SampleType string           `json:"SampleType"`
	Samples    []SwSamplesPower `json:"Samples"`
}

// SwSamplesPower holds the historical data for power wattage
type SwSamplesPower struct {
	Wattage string `json:"Wattage"`
}
