/*
 * Copyright 2024 Comcast Cable Communications Management, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package dl360

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

const (
	up2Expected = `
		 # HELP up was the last scrape of fishymetrics successful.
		 # TYPE up gauge
		 up 2
	`
	GoodDeviceInfoExpected = `
         # HELP device_info Current snapshot of device firmware information
         # TYPE device_info gauge
         device_info{biosVersion="U99 v0.00 (xx/xx/xxxx)",chassisSerialNumber="SN98765",firmwareVersion="iLO 5 v2.65",model="DL360",name="test hostname"} 1
	`
	GoodCPUStatusExpected = `
	     # HELP dl360_cpu_status Current cpu status 1 = OK, 0 = BAD
         # TYPE dl360_cpu_status gauge
         dl360_cpu_status{chassisSerialNumber="SN98765",id="1",model="cpu model",socket="Proc 1",totalCores="99"} 1
	`
	GoodLogicalDriveExpected = `
		 # HELP dl360_logical_drive_status Current Logical Drive Raid 1 = OK, 0 = BAD, -1 = DISABLED
		 # TYPE dl360_logical_drive_status gauge
		 dl360_logical_drive_status{chassisSerialNumber="SN98765",logicaldrivename="TESTDRIVE NAME 1",name="HpeSmartStorageLogicalDrive",raid="1",volumeuniqueidentifier="ABCDEF12345"} 1
	`
	GoodDiskDriveExpected = `
		 # HELP dl360_disk_drive_status Current Disk Drive status 1 = OK, 0 = BAD, -1 = DISABLED
		 # TYPE dl360_disk_drive_status gauge
		 dl360_disk_drive_status{chassisSerialNumber="SN98765",id="0",location="1I:1:1",name="HpeSmartStorageDiskDrive",serialnumber="ABC123"} 1
	`
	GoodNvmeDriveExpected = `
		 # HELP dl360_nvme_drive_status Current NVME status 1 = OK, 0 = BAD, -1 = DISABLED
		 # TYPE dl360_nvme_drive_status gauge
		 dl360_nvme_drive_status{chassisSerialNumber="SN98765",id="DA000000",protocol="NVMe",serviceLabel="Box 3:Bay 7"} 1
	`
	GoodILOSelfTestExpected = `
	     # HELP dl360_ilo_selftest_status Current ilo selftest status 1 = OK, 0 = BAD
         # TYPE dl360_ilo_selftest_status gauge
         dl360_ilo_selftest_status{chassisSerialNumber="SN98765",name="EEPROM"} 1
	`
	GoodStorageBatteryStatusExpected = `
	     # HELP dl360_storage_battery_status Current storage battery status 1 = OK, 0 = BAD
         # TYPE dl360_storage_battery_status gauge
         dl360_storage_battery_status{chassisSerialNumber="SN98765",id="1",model="battery model",name="HPE Smart Storage Battery ",serialnumber="123456789"} 1
	`
	GoodMemoryDimmExpected = `
         # HELP dl360_memory_dimm_status Current dimm status 1 = OK, 0 = BAD
         # TYPE dl360_memory_dimm_status gauge
         dl360_memory_dimm_status{capacityMiB="32768",chassisSerialNumber="SN98765",manufacturer="HPE",name="proc1dimm1",partNumber="part number",serialNumber="123456789"} 1
	`
	GoodMemoryDimmExpectedG9 = `
         # HELP dl360_memory_dimm_status Current dimm status 1 = OK, 0 = BAD
         # TYPE dl360_memory_dimm_status gauge
         dl360_memory_dimm_status{capacityMiB="32768",chassisSerialNumber="SN98765",manufacturer="HP",name="proc2dimm12",partNumber="part number",serialNumber=""} 1
	`
	GoodMemorySummaryExpected = `
	     # HELP dl360_memory_status Current memory status 1 = OK, 0 = BAD
         # TYPE dl360_memory_status gauge
         dl360_memory_status{chassisSerialNumber="SN98765",totalSystemMemoryGiB="384"} 1
	`
	GoodThermalFanSpeedExpected = `
	     # HELP dl360_thermal_fan_speed Current fan speed in the unit of percentage, possible values are 0 - 100
         # TYPE dl360_thermal_fan_speed gauge
         dl360_thermal_fan_speed{chassisSerialNumber="SN98765",name="Fan 1"} 16
	`
	GoodThermalFanStatusExpected = `
	     # HELP dl360_thermal_fan_status Current fan status 1 = OK, 0 = BAD
         # TYPE dl360_thermal_fan_status gauge
         dl360_thermal_fan_status{chassisSerialNumber="SN98765",name="Fan 1"} 1
	`
	GoodThermalSensorStatusExpected = `
	     # HELP dl360_thermal_sensor_status Current sensor status 1 = OK, 0 = BAD
         # TYPE dl360_thermal_sensor_status gauge
         dl360_thermal_sensor_status{chassisSerialNumber="SN98765",name="01-Inlet Ambient"} 1
	`
	GoodThermalSensorTempExpected = `
	     # HELP dl360_thermal_sensor_temperature Current sensor temperature reading in Celsius
         # TYPE dl360_thermal_sensor_temperature gauge
         dl360_thermal_sensor_temperature{chassisSerialNumber="SN98765",name="01-Inlet Ambient"} 22
	`
	GoodPowerVoltageOutputExpected = `
	     # HELP dl360_power_voltage_output Power voltage output in watts
         # TYPE dl360_power_voltage_output gauge
         dl360_power_voltage_output{chassisSerialNumber="SN98765",name="PSU1_VOUT"} 12.2
	`
	GoodPowerVoltageStatusExpected = `
	     # HELP dl360_power_voltage_status Current power voltage status 1 = OK, 0 = BAD
         # TYPE dl360_power_voltage_status gauge
         dl360_power_voltage_status{chassisSerialNumber="SN98765",name="PSU1_VOUT"} 1
	`
	GoodPowerSupplyOutputExpected = `
	     # HELP dl360_power_supply_output Power supply output in watts
         # TYPE dl360_power_supply_output gauge
         dl360_power_supply_output{bayNumber="1",chassisSerialNumber="SN98765",manufacturer="DELTA",model="psmodel",name="HpeServerPowerSupply",partNumber="part number",powerSupplyType="AC",serialNumber="123456789"} 91
	`
	GoodPowerSupplyStatusExpected = `
	     # HELP dl360_power_supply_status Current power supply status 1 = OK, 0 = BAD
         # TYPE dl360_power_supply_status gauge
         dl360_power_supply_status{bayNumber="1",chassisSerialNumber="SN98765",manufacturer="DELTA",model="psmodel",name="HpeServerPowerSupply",partNumber="part number",powerSupplyType="AC",serialNumber="123456789"} 1
	`
	GoodPowerSupplyTotalConsumedExpected = `
	     # HELP dl360_power_supply_total_consumed Total output of all power supplies in watts
         # TYPE dl360_power_supply_total_consumed gauge
         dl360_power_supply_total_consumed{chassisSerialNumber="SN98765",memberId="0"} 206
	`
)

type TestErrorResponse struct {
	Error TestError `json:"error"`
}

type TestError struct {
	Code         string        `json:"code"`
	Message      string        `json:"message"`
	ExtendedInfo []TestMessage `json:"@Message.ExtendedInfo"`
}

type TestMessage struct {
	MessageId string `json:"MessageId"`
}

func MustMarshal(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func Test_DL360_Exporter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redfish/v1/badcred/Managers/1/" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write(MustMarshal(TestErrorResponse{
				Error: TestError{
					Code:    "iLO.0.10.ExtendedInfo",
					Message: "See @Message.ExtendedInfo for more information.",
					ExtendedInfo: []TestMessage{
						{
							MessageId: "Base.1.0.NoValidSession",
						},
					},
				},
			}))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Unknown path - please create test case(s) for it"))
	}))
	defer server.Close()

	ctx := context.Background()
	assert := assert.New(t)

	tests := []struct {
		name       string
		uri        string
		metricName string
		metricRef1 string
		metricRef2 string
		payload    []byte
		expected   string
	}{
		{
			name:       "Bad Credentials",
			uri:        "/redfish/v1/badcred",
			metricName: "up",
			metricRef1: "up",
			metricRef2: "up",
			expected:   up2Expected,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var exporter prometheus.Collector
			var err error
			exporter, err = NewExporter(ctx, server.URL, test.uri, "")
			assert.Nil(err)
			assert.NotNil(exporter)

			prometheus.MustRegister(exporter)

			metric := (*exporter.(*Exporter).deviceMetrics)[test.metricRef1]
			m := (*metric)[test.metricRef2]

			assert.Empty(testutil.CollectAndCompare(m, strings.NewReader(test.expected), test.metricName))

			prometheus.Unregister(exporter)

		})
	}
}

// Test_DL360_Upper_Lower_Links tests the uppercase and lowercase Links/links struct because of
// the different firmware versions of the redfish API
func Test_DL360_Upper_Lower_Links(t *testing.T) {
	// server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	// 	if r.URL.Path == "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/" {
	// 		w.WriteHeader(http.StatusOK)
	// 		w.Write(MustMarshal(struct {
	// 			MembersCount int `json:"Members@odata.count"`
	// 			Members      []struct {
	// 				URL string `json:"@odata.id"`
	// 			} `json:"Members"`
	// 		}{
	// 			MembersCount: 2,
	// 			Members: []struct {
	// 				URL string `json:"@odata.id"`
	// 			}{
	// 				{
	// 					URL: "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/0/",
	// 				},
	// 				{
	// 					URL: "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/2/",
	// 				},
	// 			},
	// 		}))
	// 		return
	// 	} else if r.URL.Path == "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/0/" {
	// 		w.WriteHeader(http.StatusOK)
	// 		w.Write(MustMarshal(struct {
	// 			LinksUpper struct {
	// 				LogicalDrives struct {
	// 					URL string `json:"@odata.id"`
	// 				} `json:"LogicalDrives"`
	// 				PhysicalDrives struct {
	// 					URL string `json:"@odata.id"`
	// 				} `json:"PhysicalDrives"`
	// 			} `json:"Links"`
	// 		}{
	// 			LinksUpper: struct {
	// 				LogicalDrives struct {
	// 					URL string `json:"@odata.id"`
	// 				} `json:"LogicalDrives"`
	// 				PhysicalDrives struct {
	// 					URL string `json:"@odata.id"`
	// 				} `json:"PhysicalDrives"`
	// 			}{
	// 				LogicalDrives: struct {
	// 					URL string `json:"@odata.id"`
	// 				}{
	// 					URL: "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/0/LogicalDrives/",
	// 				},
	// 				PhysicalDrives: struct {
	// 					URL string `json:"@odata.id"`
	// 				}{
	// 					URL: "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/0/DiskDrives/",
	// 				},
	// 			},
	// 		}))
	// 		return
	// 	} else if r.URL.Path == "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/2/" {
	// 		w.WriteHeader(http.StatusOK)
	// 		w.Write(MustMarshal(struct {
	// 			LinksLower struct {
	// 				LogicalDrives struct {
	// 					URL string `json:"href"`
	// 				} `json:"LogicalDrives"`
	// 				PhysicalDrives struct {
	// 					URL string `json:"href"`
	// 				} `json:"PhysicalDrives"`
	// 			} `json:"links"`
	// 		}{
	// 			LinksLower: struct {
	// 				LogicalDrives struct {
	// 					URL string `json:"href"`
	// 				} `json:"LogicalDrives"`
	// 				PhysicalDrives struct {
	// 					URL string `json:"href"`
	// 				} `json:"PhysicalDrives"`
	// 			}{
	// 				LogicalDrives: struct {
	// 					URL string `json:"href"`
	// 				}{
	// 					URL: "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/2/LogicalDrives/",
	// 				},
	// 				PhysicalDrives: struct {
	// 					URL string `json:"href"`
	// 				}{
	// 					URL: "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/2/DiskDrives/",
	// 				},
	// 			},
	// 		}))
	// 		return
	// 	} else if r.URL.Path == "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/0/LogicalDrives/" {
	// 		w.WriteHeader(http.StatusOK)
	// 		w.Write(MustMarshal(struct {
	// 			MembersCount int `json:"Members@odata.count"`
	// 			Members      []struct {
	// 				URL string `json:"@odata.id"`
	// 			} `json:"Members"`
	// 		}{
	// 			MembersCount: 1,
	// 			Members: []struct {
	// 				URL string `json:"@odata.id"`
	// 			}{
	// 				{
	// 					URL: "/redfish/v1/Systems/1/SmartStorage/ArrayControllers/0/LogicalDrives/1/",
	// 				},
	// 			},
	// 		}))
	// 		return
	// 	} else if r.URL.Path == "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/2/LogicalDrives/" {
	// 		w.WriteHeader(http.StatusOK)
	// 		w.Write(MustMarshal(struct {
	// 			MembersCount int `json:"Members@odata.count"`
	// 			Members      []struct {
	// 				URL string `json:"@odata.id"`
	// 			} `json:"Members"`
	// 		}{
	// 			MembersCount: 1,
	// 			Members: []struct {
	// 				URL string `json:"@odata.id"`
	// 			}{
	// 				{
	// 					URL: "/redfish/v1/Systems/1/SmartStorage/ArrayControllers/2/LogicalDrives/1/",
	// 				},
	// 			},
	// 		}))
	// 		return
	// 	} else if r.URL.Path == "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/0/DiskDrives/" {
	// 		w.WriteHeader(http.StatusOK)
	// 		w.Write(MustMarshal(struct {
	// 			MembersCount int `json:"Members@odata.count"`
	// 			Members      []struct {
	// 				URL string `json:"@odata.id"`
	// 			} `json:"Members"`
	// 		}{
	// 			MembersCount: 1,
	// 			Members: []struct {
	// 				URL string `json:"@odata.id"`
	// 			}{
	// 				{
	// 					URL: "/redfish/v1/Systems/1/SmartStorage/ArrayControllers/0/DiskDrives/0/",
	// 				},
	// 			},
	// 		}))
	// 		return
	// 	} else if r.URL.Path == "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/2/DiskDrives/" {
	// 		w.WriteHeader(http.StatusOK)
	// 		w.Write(MustMarshal(struct {
	// 			MembersCount int `json:"Members@odata.count"`
	// 			Members      []struct {
	// 				URL string `json:"@odata.id"`
	// 			} `json:"Members"`
	// 		}{
	// 			MembersCount: 1,
	// 			Members: []struct {
	// 				URL string `json:"@odata.id"`
	// 			}{
	// 				{
	// 					URL: "/redfish/v1/Systems/1/SmartStorage/ArrayControllers/2/DiskDrives/0/",
	// 				},
	// 			},
	// 		}))
	// 		return
	// 	} else if r.URL.Path == "/redfish/v1/good/Chassis/1/" {
	// 		w.WriteHeader(http.StatusOK)
	// 		w.Write(MustMarshal(struct {
	// 			LinksUpper struct {
	// 				Drives []struct {
	// 					URL string `json:"@odata.id"`
	// 				} `json:"Drives"`
	// 			} `json:"Links"`
	// 		}{
	// 			LinksUpper: struct {
	// 				Drives []struct {
	// 					URL string `json:"@odata.id"`
	// 				} `json:"Drives"`
	// 			}{
	// 				Drives: []struct {
	// 					URL string `json:"@odata.id"`
	// 				}{
	// 					{
	// 						URL: "/redfish/v1/Systems/1/Storage/DA000000/Drives/DA000000/",
	// 					},
	// 				},
	// 			},
	// 		}))
	// 		return
	// 	}
	// 	w.WriteHeader(http.StatusInternalServerError)
	// 	w.Write([]byte("Unknown path - please create test case(s) for it"))
	// }))
	// defer server.Close()
}

func Test_DL360_Metrics_Handling(t *testing.T) {

	var GoodDeviceInfoResponse = []byte(`{
			"Description": "test description",
  			"FirmwareVersion": "iLO 5 v2.65",
  			"Model": "iLO 5"
  		}`)
	var GoodCPUStatusResponse = []byte(`{
  			"Id": "1",
  			"Model": "cpu model",
  			"Name": "Processors",
  			"ProcessorArchitecture": "x86",
  			"Socket": "Proc 1",
  			"Status": {
  			  "Health": "OK",
  			  "State": "Enabled"
  			},
  			"TotalCores": 99,
  			"TotalThreads": 99
  		}`)
	var GoodLogicalDriveResponse = []byte(`{
  			"Id": "1",
  			"CapacityMiB": 915683,
  			"Description": "HPE Smart Storage Logical Drive View",
  			"InterfaceType": "SATA",
  			"LogicalDriveName": "TESTDRIVE NAME 1",
  			"LogicalDriveNumber": 1,
  			"Name": "HpeSmartStorageLogicalDrive",
  			"Raid": "1",
  			"Status": {
  			  "Health": "OK",
  			  "State": "Enabled"
  			},
  			"StripeSizeBytes": 262144,
  			"VolumeUniqueIdentifier": "ABCDEF12345"
		}`)
	var GoodDiskDriveResponse = []byte(`{
  			"Id": "0",
  			"CapacityMiB": 915715,
  			"Description": "HPE Smart Storage Disk Drive View",
  			"InterfaceType": "SATA",
  			"Location": "1I:1:1",
  			"Model": "model name",
  			"Name": "HpeSmartStorageDiskDrive",
  			"SerialNumber": "ABC123",
  			"Status": {
  			  "Health": "OK",
  			  "State": "Enabled"
  			}
		}`)
	var GoodNvmeDriveResponse = []byte(`{
  			"Id": "DA000000",
  			"CapacityBytes": 1600321314816,
  			"FailurePredicted": false,
  			"MediaType": "SSD",
  			"Model": "model name",
  			"Name": "Secondary Storage Device",
  			"Oem": {
  			  "Hpe": {
  			    "CurrentTemperatureCelsius": 33,
  			    "DriveStatus": {
  			      "Health": "OK",
  			      "State": "Enabled"
  			    },
  			    "NVMeId": "drive id"
  			  }
  			},
  			"PhysicalLocation": {
  			  "PartLocation": {
  			    "ServiceLabel": "Box 3:Bay 7"
  			  }
  			},
  			"Protocol": "NVMe"
		}`)
	var GoodILOSelfTestResponse = []byte(`{
  			"Oem": {
  			  "Hpe": {
  			    "iLOSelfTestResults": [
  			      {
  			        "Notes": "",
  			        "SelfTestName": "EEPROM",
  			        "Status": "OK"
  			      }
  			    ]
  			  }
  			}
  		}`)
	var GoodStorageBatteryStatusResponse = []byte(`{
  			"Oem": {
  			  "Hp": {
  			    "Battery": [
  			      {
  			        "Condition": "Ok",
  			        "ErrorCode": 0,
  			        "FirmwareVersion": "1.1",
  			        "Index": 1,
  			        "MaxCapWatts": 96,
  			        "Model": "battery model",
  			        "Present": "Yes",
  			        "ProductName": "HPE Smart Storage Battery ",
  			        "SerialNumber": "123456789",
  			        "Spare": "815983-001"
  			      }
  			    ]
  			  }
  			}
  		}`)
	var GoodMemoryDimmResponse = []byte(`{
  			"CapacityMiB": 32768,
  			"Manufacturer": "HPE",
  			"MemoryDeviceType": "DDR4",
  			"Name": "proc1dimm1",
  			"PartNumber": "part number  ",
  			"SerialNumber": "123456789",
  			"Status": {
  			  "Health": "OK",
  			  "State": "Enabled"
  			}
  		}`)
	var GoodMemoryDimmResponseG9 = []byte(`{
				"DIMMStatus": "GoodInUse",
				"Name": "proc2dimm12",
				"SizeMB": 32768,
				"Manufacturer": "HP     ",
				"PartNumber": "part number",
				"DIMMType": "DDR4"
			}`)
	var GoodMemorySummaryResponse = []byte(`{
  			"Id": "1",
  			"MemorySummary": {
  			  "Status": {
  			    "HealthRollup": "OK"
  			  },
  			  "TotalSystemMemoryGiB": 384,
  			  "TotalSystemPersistentMemoryGiB": 0
  			}
  		}`)
	var GoodThermalFanSpeedResponse = []byte(`{
			"@odata.id": "/redfish/v1/Chassis/1/Thermal/",
  			"Id": "Thermal",
  			"Fans": [
  			  {
  			    "MemberId": "0",
  			    "Name": "Fan 1",
  			    "Reading": 16,
  			    "ReadingUnits": "Percent",
  			    "Status": {
  			      "Health": "OK",
  			      "State": "Enabled"
  			    }
  			  }
  			],
  			"Name": "Thermal",
  			"Temperatures": [
  			  {
  			    "MemberId": "0",
  			    "Name": "01-Inlet Ambient",
  			    "PhysicalContext": "Intake",
  			    "ReadingCelsius": 22,
  			    "SensorNumber": 1,
  			    "Status": {
  			      "Health": "OK",
  			      "State": "Enabled"
  			    },
  			    "UpperThresholdCritical": 42,
  			    "UpperThresholdFatal": 47
  			  }
  			]
  		}`)
	var GoodThermalFanStatusResponse = []byte(`{
			"@odata.id": "/redfish/v1/Chassis/1/Thermal/",
  			"Id": "Thermal",
  			"Fans": [
  			  {
  			    "MemberId": "0",
  			    "Name": "Fan 1",
  			    "Reading": 18,
  			    "ReadingUnits": "Percent",
  			    "Status": {
  			      "Health": "OK",
  			      "State": "Enabled"
  			    }
  			  }
  			],
  			"Name": "Thermal",
  			"Temperatures": [
  			  {
  			    "MemberId": "0",
  			    "Name": "01-Inlet Ambient",
  			    "PhysicalContext": "Intake",
  			    "ReadingCelsius": 22,
  			    "SensorNumber": 1,
  			    "Status": {
  			      "Health": "OK",
  			      "State": "Enabled"
  			    },
  			    "UpperThresholdCritical": 42,
  			    "UpperThresholdFatal": 47
  			  }
  			]
  		}`)
	var GoodThermalSensorStatusResponse = []byte(`{
			"@odata.id": "/redfish/v1/Chassis/1/Thermal/",
  			"Id": "Thermal",
  			"Fans": [
  			  {
  			    "MemberId": "0",
  			    "Name": "Fan 1",
  			    "Reading": 18,
  			    "ReadingUnits": "Percent",
  			    "Status": {
  			      "Health": "OK",
  			      "State": "Enabled"
  			    }
  			  }
  			],
  			"Name": "Thermal",
  			"Temperatures": [
  			  {
  			    "MemberId": "0",
  			    "Name": "01-Inlet Ambient",
  			    "PhysicalContext": "Intake",
  			    "ReadingCelsius": 22,
  			    "SensorNumber": 1,
  			    "Status": {
  			      "Health": "OK",
  			      "State": "Enabled"
  			    },
  			    "UpperThresholdCritical": 42,
  			    "UpperThresholdFatal": 47
  			  }
  			]
  		}`)
	var GoodThermalSensorTempResponse = []byte(`{
			"@odata.id": "/redfish/v1/Chassis/1/Thermal/",
  			"Id": "Thermal",
  			"Fans": [
  			  {
  			    "MemberId": "0",
  			    "Name": "Fan 1",
  			    "Reading": 18,
  			    "ReadingUnits": "Percent",
  			    "Status": {
  			      "Health": "OK",
  			      "State": "Enabled"
  			    }
  			  }
  			],
  			"Name": "Thermal",
  			"Temperatures": [
  			  {
  			    "MemberId": "0",
  			    "Name": "01-Inlet Ambient",
  			    "PhysicalContext": "Intake",
  			    "ReadingCelsius": 22,
  			    "SensorNumber": 1,
  			    "Status": {
  			      "Health": "OK",
  			      "State": "Enabled"
  			    },
  			    "UpperThresholdCritical": 42,
  			    "UpperThresholdFatal": 47
  			  }
  			]
  		}`)
	var GoodPowerVoltageOutputResponse = []byte(`{
  			"PowerControl": [
  			  {
  			    "PhysicalContext": "PowerSupply",
  			    "PowerMetrics": {
  			      "MinConsumedWatts": 266,
  			      "AverageConsumedWatts": 358,
  			      "MaxConsumedWatts": 614
  			    },
  			    "MemberId": "1",
  			    "PowerConsumedWatts": 378
  			  }
  			],
  			"Voltages": [
  			  {
  			    "Status": {
  			      "State": "Enabled",
  			      "Health": "OK"
  			    },
  			    "UpperThresholdCritical": 14,
  			    "Name": "PSU1_VOUT",
  			    "ReadingVolts": 12.2
  			  }
  			],
  			"Id": "Power",
  			"PowerSupplies": [
  			  {
  			    "SerialNumber": "123456789",
  			    "InputRanges": [
  			      {
  			        "InputType": "AC",
  			        "OutputWattage": 1050
  			      }
  			    ],
  			    "FirmwareVersion": "123",
  			    "PowerOutputWatts": 160,
  			    "LineInputVoltage": 202,
  			    "Name": "PSU1",
  			    "Status": {
  			      "State": "Enabled"
  			    },
  			    "PowerInputWatts": 180,
  			    "Manufacturer": "DELTA",
  			    "LastPowerOutputWatts": 160,
  			    "MemberId": "1",
  			    "PartNumber": "part number",
  			    "PowerSupplyType": "AC",
  			    "Model": "psmodel",
  			    "SparePartNumber": "part number"
  			  }
  			],
  			"Name": "Power"
  		}`)
	var GoodPowerVoltageStatusResponse = []byte(`{
  			"PowerControl": [
  			  {
  			    "PhysicalContext": "PowerSupply",
  			    "PowerMetrics": {
  			      "MinConsumedWatts": 266,
  			      "AverageConsumedWatts": 358,
  			      "MaxConsumedWatts": 614
  			    },
  			    "MemberId": "1",
  			    "PowerConsumedWatts": 378
  			  }
  			],
  			"Voltages": [
  			  {
  			    "Status": {
  			      "State": "Enabled",
  			      "Health": "OK"
  			    },
  			    "UpperThresholdCritical": 14,
  			    "Name": "PSU1_VOUT",
  			    "ReadingVolts": 12.2
  			  }
  			],
  			"Id": "Power",
  			"PowerSupplies": [
  			  {
  			    "SerialNumber": "123456789",
  			    "InputRanges": [
  			      {
  			        "InputType": "AC",
  			        "OutputWattage": 1050
  			      }
  			    ],
  			    "FirmwareVersion": "123",
  			    "PowerOutputWatts": 160,
  			    "LineInputVoltage": 202,
  			    "Name": "PSU1",
  			    "Status": {
  			      "State": "Enabled"
  			    },
  			    "PowerInputWatts": 180,
  			    "Manufacturer": "DELTA",
  			    "LastPowerOutputWatts": 160,
  			    "MemberId": "1",
  			    "PartNumber": "part number",
  			    "PowerSupplyType": "AC",
  			    "Model": "psmodel",
  			    "SparePartNumber": "part number"
  			  }
  			],
  			"Name": "Power"
  		}`)
	var GoodPowerSupplyOutputResponse = []byte(`{
  			"Id": "Power",
  			"Name": "PowerMetrics",
  			"PowerControl": [
  			  {
  			    "@odata.id": "/redfish/v1/Chassis/1/Power/#PowerControl/0",
  			    "MemberId": "0",
  			    "PowerCapacityWatts": 1600,
  			    "PowerConsumedWatts": 206,
  			    "PowerMetrics": {
  			      "AverageConsumedWatts": 207,
  			      "IntervalInMin": 20,
  			      "MaxConsumedWatts": 282,
  			      "MinConsumedWatts": 205
  			    }
  			  }
  			],
  			"PowerSupplies": [
  			  {
  			    "@odata.id": "/redfish/v1/Chassis/1/Power/#PowerSupplies/0",
  			    "FirmwareVersion": "2.04",
  			    "LastPowerOutputWatts": 91,
  			    "LineInputVoltage": 206,
  			    "LineInputVoltageType": "ACHighLine",
  			    "Manufacturer": "DELTA",
  			    "MemberId": "0",
  			    "Model": "psmodel",
  			    "Name": "HpeServerPowerSupply",
  			    "Oem": {
  			      "Hpe": {
  			        "AveragePowerOutputWatts": 91,
  			        "BayNumber": 1,
  			        "HotplugCapable": true,
  			        "MaxPowerOutputWatts": 93,
  			        "Mismatched": false,
  			        "PowerSupplyStatus": {
  			          "State": "Ok"
  			        },
  			        "iPDUCapable": false
  			      }
  			    },
  			    "PowerCapacityWatts": 800,
  			    "PowerSupplyType": "AC",
  			    "SerialNumber": "123456789",
  			    "SparePartNumber": "part number",
  			    "Status": {
  			      "Health": "OK",
  			      "State": "Enabled"
  			    }
  			  }
  			]
  		}`)
	var GoodPowerSupplyStatusResponse = []byte(`{
  			"Id": "Power",
  			"Name": "PowerMetrics",
  			"PowerControl": [
  			  {
  			    "@odata.id": "/redfish/v1/Chassis/1/Power/#PowerControl/0",
  			    "MemberId": "0",
  			    "PowerCapacityWatts": 1600,
  			    "PowerConsumedWatts": 206,
  			    "PowerMetrics": {
  			      "AverageConsumedWatts": 207,
  			      "IntervalInMin": 20,
  			      "MaxConsumedWatts": 282,
  			      "MinConsumedWatts": 205
  			    }
  			  }
  			],
  			"PowerSupplies": [
  			  {
  			    "@odata.id": "/redfish/v1/Chassis/1/Power/#PowerSupplies/0",
  			    "FirmwareVersion": "2.04",
  			    "LastPowerOutputWatts": 91,
  			    "LineInputVoltage": 206,
  			    "LineInputVoltageType": "ACHighLine",
  			    "Manufacturer": "DELTA",
  			    "MemberId": "0",
  			    "Model": "psmodel",
  			    "Name": "HpeServerPowerSupply",
  			    "Oem": {
  			      "Hpe": {
  			        "AveragePowerOutputWatts": 91,
  			        "BayNumber": 1,
  			        "HotplugCapable": true,
  			        "MaxPowerOutputWatts": 93,
  			        "Mismatched": false,
  			        "PowerSupplyStatus": {
  			          "State": "Ok"
  			        },
  			        "iPDUCapable": false
  			      }
  			    },
  			    "PowerCapacityWatts": 800,
  			    "PowerSupplyType": "AC",
  			    "SerialNumber": "123456789",
  			    "SparePartNumber": "part number",
  			    "Status": {
  			      "Health": "OK",
  			      "State": "Enabled"
  			    }
  			  }
  			]
  		}`)
	var GoodPowerSupplyTotalConsumedResponse = []byte(`{
  			"Id": "Power",
  			"Name": "PowerMetrics",
  			"PowerControl": [
  			  {
  			    "@odata.id": "/redfish/v1/Chassis/1/Power/#PowerControl/0",
  			    "MemberId": "0",
  			    "PowerCapacityWatts": 1600,
  			    "PowerConsumedWatts": 206,
  			    "PowerMetrics": {
  			      "AverageConsumedWatts": 207,
  			      "IntervalInMin": 20,
  			      "MaxConsumedWatts": 282,
  			      "MinConsumedWatts": 205
  			    }
  			  }
  			],
  			"PowerSupplies": [
  			  {
  			    "@odata.id": "/redfish/v1/Chassis/1/Power/#PowerSupplies/0",
  			    "FirmwareVersion": "2.04",
  			    "LastPowerOutputWatts": 91,
  			    "LineInputVoltage": 206,
  			    "LineInputVoltageType": "ACHighLine",
  			    "Manufacturer": "DELTA",
  			    "MemberId": "0",
  			    "Model": "psmodel",
  			    "Name": "HpeServerPowerSupply",
  			    "Oem": {
  			      "Hpe": {
  			        "AveragePowerOutputWatts": 91,
  			        "BayNumber": 1,
  			        "HotplugCapable": true,
  			        "MaxPowerOutputWatts": 93,
  			        "Mismatched": false,
  			        "PowerSupplyStatus": {
  			          "State": "Ok"
  			        },
  			        "iPDUCapable": false
  			      }
  			    },
  			    "PowerCapacityWatts": 800,
  			    "PowerSupplyType": "AC",
  			    "SerialNumber": "123456789",
  			    "SparePartNumber": "part number",
  			    "Status": {
  			      "Health": "OK",
  			      "State": "Enabled"
  			    }
  			  }
  			]
  		}`)

	var exporter prometheus.Collector

	assert := assert.New(t)

	metrx := NewDeviceMetrics()

	exporter = &Exporter{
		ctx:                 context.Background(),
		host:                "fishymetrics.com",
		biosVersion:         "U99 v0.00 (xx/xx/xxxx)",
		chassisSerialNumber: "SN98765",
		iloServerName:       "test hostname",
		deviceMetrics:       metrx,
	}

	prometheus.MustRegister(exporter)

	deviceInfoMetrics := func(exp *Exporter, payload []byte) error {
		err := exp.exportFirmwareMetrics(payload)
		if err != nil {
			return err
		}
		return nil
	}

	processorMetrics := func(exp *Exporter, payload []byte) error {
		err := exp.exportProcessorMetrics(payload)
		if err != nil {
			return err
		}
		return nil
	}

	logicalDevMetrics := func(exp *Exporter, payload []byte) error {
		err := exp.exportLogicalDriveMetrics(payload)
		if err != nil {
			return err
		}
		return nil
	}

	physDevMetrics := func(exp *Exporter, payload []byte) error {
		err := exp.exportPhysicalDriveMetrics(payload)
		if err != nil {
			return err
		}
		return nil
	}

	nvmeDevMetrics := func(exp *Exporter, payload []byte) error {
		err := exp.exportNVMeDriveMetrics(payload)
		if err != nil {
			return err
		}
		return nil
	}

	iloSelfTestMetrics := func(exp *Exporter, payload []byte) error {
		err := exp.exportIloSelfTest(payload)
		if err != nil {
			return err
		}
		return nil
	}

	storBatterytMetrics := func(exp *Exporter, payload []byte) error {
		err := exp.exportStorageBattery(payload)
		if err != nil {
			return err
		}
		return nil
	}

	memDimmMetrics := func(exp *Exporter, payload []byte) error {
		err := exp.exportMemoryMetrics(payload)
		if err != nil {
			return err
		}
		return nil
	}

	memSummaryMetrics := func(exp *Exporter, payload []byte) error {
		err := exp.exportMemorySummaryMetrics(payload)
		if err != nil {
			return err
		}
		return nil
	}

	thermMetrics := func(exp *Exporter, payload []byte) error {
		err := exp.exportThermalMetrics(payload)
		if err != nil {
			return err
		}
		return nil
	}

	powMetrics := func(exp *Exporter, payload []byte) error {
		err := exp.exportPowerMetrics(payload)
		if err != nil {
			return err
		}
		return nil
	}

	tests := []struct {
		name       string
		metricName string
		metricRef1 string
		metricRef2 string
		handleFunc func(*Exporter, []byte) error
		response   []byte
		expected   string
	}{
		{
			name:       "Good Device Info",
			metricName: "device_info",
			metricRef1: "deviceInfo",
			metricRef2: "deviceInfo",
			handleFunc: deviceInfoMetrics,
			response:   GoodDeviceInfoResponse,
			expected:   GoodDeviceInfoExpected,
		},
		{
			name:       "Good CPU Status",
			metricName: "dl360_cpu_status",
			metricRef1: "processorMetrics",
			metricRef2: "processorStatus",
			handleFunc: processorMetrics,
			response:   GoodCPUStatusResponse,
			expected:   GoodCPUStatusExpected,
		},
		{
			name:       "Good Logical Drive",
			metricName: "dl360_logical_drive_status",
			metricRef1: "logicalDriveMetrics",
			metricRef2: "raidStatus",
			handleFunc: logicalDevMetrics,
			response:   GoodLogicalDriveResponse,
			expected:   GoodLogicalDriveExpected,
		},
		{
			name:       "Good Disk Drive",
			metricName: "dl360_disk_drive_status",
			metricRef1: "diskDriveMetrics",
			metricRef2: "driveStatus",
			handleFunc: physDevMetrics,
			response:   GoodDiskDriveResponse,
			expected:   GoodDiskDriveExpected,
		},
		{
			name:       "Good Nvme Drive",
			metricName: "dl360_nvme_drive_status",
			metricRef1: "nvmeMetrics",
			metricRef2: "nvmeDriveStatus",
			handleFunc: nvmeDevMetrics,
			response:   GoodNvmeDriveResponse,
			expected:   GoodNvmeDriveExpected,
		},
		{
			name:       "Good iLO Self Test",
			metricName: "dl360_ilo_selftest_status",
			metricRef1: "iloSelfTestMetrics",
			metricRef2: "iloSelfTestStatus",
			handleFunc: iloSelfTestMetrics,
			response:   GoodILOSelfTestResponse,
			expected:   GoodILOSelfTestExpected,
		},
		{
			name:       "Good Storage Battery Status",
			metricName: "dl360_storage_battery_status",
			metricRef1: "storBatteryMetrics",
			metricRef2: "storageBatteryStatus",
			handleFunc: storBatterytMetrics,
			response:   GoodStorageBatteryStatusResponse,
			expected:   GoodStorageBatteryStatusExpected,
		},
		{
			name:       "Good Memory DIMM Status",
			metricName: "dl360_memory_dimm_status",
			metricRef1: "memoryMetrics",
			metricRef2: "memoryDimmStatus",
			handleFunc: memDimmMetrics,
			response:   GoodMemoryDimmResponse,
			expected:   GoodMemoryDimmExpected,
		},
		{
			name:       "Good Memory DIMM Status G9",
			metricName: "dl360_memory_dimm_status",
			metricRef1: "memoryMetrics",
			metricRef2: "memoryDimmStatus",
			handleFunc: memDimmMetrics,
			response:   GoodMemoryDimmResponseG9,
			expected:   GoodMemoryDimmExpectedG9,
		},
		{
			name:       "Good Memory Summary Status",
			metricName: "dl360_memory_status",
			metricRef1: "memoryMetrics",
			metricRef2: "memoryStatus",
			handleFunc: memSummaryMetrics,
			response:   GoodMemorySummaryResponse,
			expected:   GoodMemorySummaryExpected,
		},
		{
			name:       "Good Thermal Fan Speed",
			metricName: "dl360_thermal_fan_speed",
			metricRef1: "thermalMetrics",
			metricRef2: "fanSpeed",
			handleFunc: thermMetrics,
			response:   GoodThermalFanSpeedResponse,
			expected:   GoodThermalFanSpeedExpected,
		},
		{
			name:       "Good Thermal Fan Status",
			metricName: "dl360_thermal_fan_status",
			metricRef1: "thermalMetrics",
			metricRef2: "fanStatus",
			handleFunc: thermMetrics,
			response:   GoodThermalFanStatusResponse,
			expected:   GoodThermalFanStatusExpected,
		},
		{
			name:       "Good Thermal Sensor Status",
			metricName: "dl360_thermal_sensor_status",
			metricRef1: "thermalMetrics",
			metricRef2: "sensorStatus",
			handleFunc: thermMetrics,
			response:   GoodThermalSensorStatusResponse,
			expected:   GoodThermalSensorStatusExpected,
		},
		{
			name:       "Good Thermal Sensor Temperature",
			metricName: "dl360_thermal_sensor_temperature",
			metricRef1: "thermalMetrics",
			metricRef2: "sensorTemperature",
			handleFunc: thermMetrics,
			response:   GoodThermalSensorTempResponse,
			expected:   GoodThermalSensorTempExpected,
		},
		{
			name:       "Good Power Voltage Output",
			metricName: "dl360_power_voltage_output",
			metricRef1: "powerMetrics",
			metricRef2: "voltageOutput",
			handleFunc: powMetrics,
			response:   GoodPowerVoltageOutputResponse,
			expected:   GoodPowerVoltageOutputExpected,
		},
		{
			name:       "Good Power Voltage Status",
			metricName: "dl360_power_voltage_status",
			metricRef1: "powerMetrics",
			metricRef2: "voltageStatus",
			handleFunc: powMetrics,
			response:   GoodPowerVoltageStatusResponse,
			expected:   GoodPowerVoltageStatusExpected,
		},
		{
			name:       "Good Power Supply Output",
			metricName: "dl360_power_supply_output",
			metricRef1: "powerMetrics",
			metricRef2: "supplyOutput",
			handleFunc: powMetrics,
			response:   GoodPowerSupplyOutputResponse,
			expected:   GoodPowerSupplyOutputExpected,
		},
		{
			name:       "Good Power Supply Status",
			metricName: "dl360_power_supply_status",
			metricRef1: "powerMetrics",
			metricRef2: "supplyStatus",
			handleFunc: powMetrics,
			response:   GoodPowerSupplyStatusResponse,
			expected:   GoodPowerSupplyStatusExpected,
		},
		{
			name:       "Good Power Supply Total Consumed",
			metricName: "dl360_power_supply_total_consumed",
			metricRef1: "powerMetrics",
			metricRef2: "supplyTotalConsumed",
			handleFunc: powMetrics,
			response:   GoodPowerSupplyTotalConsumedResponse,
			expected:   GoodPowerSupplyTotalConsumedExpected,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// clear metric before each test
			metric := (*exporter.(*Exporter).deviceMetrics)[test.metricRef1]
			m := (*metric)[test.metricRef2]
			m.Reset()

			err := test.handleFunc(exporter.(*Exporter), test.response)
			if err != nil {
				t.Error(err)
			}

			assert.Empty(testutil.CollectAndCompare(m, strings.NewReader(test.expected), test.metricName))
		})
	}
	prometheus.Unregister(exporter)
}
