package c220

import (
	"bytes"
	"encoding/json"
)

// JSON
// /redfish/v1/Chassis/1/Power/

// PowerMetrics is the top level json object for Power metadata
type PowerMetrics struct {
	Name          string              `json:"Name"`
	PowerControl  PowerControlWrapper `json:"PowerControl"`
	PowerSupplies []PowerSupply       `json:"PowerSupplies"`
	Voltages      []Voltages          `json:"Voltages"`
	Url           string              `json:"@odata.id"`
}

// PowerControl is the top level json object for metadata on power supply consumption
type PowerControl struct {
	PowerConsumedWatts interface{} `json:"PowerConsumedWatts"`
	PowerMetrics       PowerMetric `json:"PowerMetrics"`
}

type PowerControlSlice struct {
	PowerControl []PowerControl
}

type PowerControlWrapper struct {
	PowerControlSlice
}

func (w *PowerControlWrapper) UnmarshalJSON(data []byte) error {
	// because of a change in output betwen c220 firmware versions we need to account for this
	if bytes.Compare([]byte("{"), data[0:1]) == 0 {
		var powCtlSlice PowerControl
		err := json.Unmarshal(data, &powCtlSlice)
		if err != nil {
			return err
		}
		s := make([]PowerControl, 0)
		s = append(s, powCtlSlice)
		w.PowerControl = s
		return nil
	}
	return json.Unmarshal(data, &w.PowerControl)
}

// PowerMetric contains avg/min/max power metadata
type PowerMetric struct {
	AverageConsumedWatts interface{} `json:"AverageConsumedWatts"`
	MaxConsumedWatts     interface{} `json:"MaxConsumedWatts"`
	MinConsumedWatts     interface{} `json:"MinConsumedWatts"`
}

// Voltages contains current/lower/upper voltage and power supply status metadata
type Voltages struct {
	Name                   string      `json:"Name"`
	ReadingVolts           interface{} `json:"ReadingVolts"`
	Status                 Status      `json:"Status"`
	UpperThresholdCritical interface{} `json:"UpperThresholdCritical"`
}

// PowerSupply is the top level json object for metadata on power supply product info
type PowerSupply struct {
	FirmwareVersion      string       `json:"FirmwareVersion"`
	LastPowerOutputWatts interface{}  `json:"LastPowerOutputWatts"`
	LineInputVoltage     interface{}  `json:"LineInputVoltage"`
	LineInputVoltageType string       `json:"LineInputVoltageType"`
	InputRanges          []InputRange `json:"InputRanges,omitempty"`
	Manufacturer         string       `json:"Manufacturer"`
	Model                string       `json:"Model"`
	Name                 string       `json:"Name"`
	PartNumber           string       `json:"PartNumber"`
	PowerSupplyType      string       `json:"PowerSupplyType"`
	SerialNumber         string       `json:"SerialNumber"`
	SparePartNumber      string       `json:"SparePartNumber"`
	Status               Status       `json:"Status"`
}

// InputRange is the top level json object for input voltage metadata
type InputRange struct {
	InputType     string `json:"InputType"`
	OutputWattage int    `json:"OutputWattage"`
}
