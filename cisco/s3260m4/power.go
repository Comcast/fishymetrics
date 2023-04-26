package s3260m4

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
)

// XML classId='equipmentPsu'

type XMLPowerMetrics struct {
	XMLName    xml.Name   `xml:"configResolveClass"`
	OutConfigs OutConfigs `xml:"outConfigs"`
}

type OutConfigs struct {
	XMLName xml.Name       `xml:"outConfigs"`
	PSUs    []EquipmentPsu `xml:"equipmentPsu"`
}

type EquipmentPsu struct {
	ID              string `xml:"id,attr"`
	Name            string `xml:"dn,attr"`
	Pid             string `xml:"pid,attr"`
	Model           string `xml:"mode,attr"`
	Operability     string `xml:"operability,attr"`
	Power           string `xml:"power,attr"`
	Presence        string `xml:"presence,attr"`
	Serial          string `xml:"serial,attr"`
	Thermal         string `xml:"thermal,attr"`
	Voltage         string `xml:"voltage,attr"`
	Input           string `xml:"input,attr"`
	MaxOutput       string `xml:"maxOutput,attr"`
	FirmwareVersion string `xml:"fwVersion,attr"`
}

// JSON
// /redfish/v1/Chassis/XXXXX/Power/

// PowerMetrics is the top level json object for Power metadata
type PowerMetrics struct {
	Name          string              `json:"Name"`
	PowerControl  PowerControlWrapper `json:"PowerControl"`
	PowerSupplies []PowerSupply       `json:"PowerSupplies,omitempty"`
	Voltages      []Voltages          `json:"Voltages"`
	Url           string              `json:"@odata.id"`
}

type PowerControlSlice struct {
	PowerControl []PowerControl
}

type PowerControlWrapper struct {
	PowerControlSlice
}

func (w *PowerControlWrapper) UnmarshalJSON(data []byte) error {
	// because of a change in output betwen s3260m4 firmware versions we need to account for this
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

// PowerControl is the top level json object for metadata on power supply consumption
type PowerControl struct {
	PowerLimit         PowerLimitWrapper `json:"PowerLimit"`
	PowerConsumedWatts interface{}       `json:"PowerConsumedWatts,omitempty"`
	PowerMetrics       PowerMetric       `json:"PowerMetrics"`
}

type PowerLimit struct {
	LimitInWatts interface{} `json:"LimitInWatts,omitempty"`
}

type PowerLimitWrapper struct {
	PowerLimit
}

func (w *PowerLimitWrapper) UnmarshalJSON(data []byte) error {
	if bytes.Compare([]byte("[]"), data) == 0 {
		w.PowerLimit = PowerLimit{}
		return nil
	}
	return json.Unmarshal(data, &w.PowerLimit)
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
	PowerSupplyType      string       `json:"PowerSupplyType"`
	SerialNumber         string       `json:"SerialNumber"`
	SparePartNumber      string       `json:"SparePartNumber"`
	Status               struct {
		State string `json:"state"`
	} `json:"Status"`
}

// InputRange is the top level json object for input voltage metadata
type InputRange struct {
	InputType     string `json:"InputType"`
	OutputWattage int    `json:"OutputWattage"`
}
