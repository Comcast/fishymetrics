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

package exporter

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
        # HELP redfish_device_info Current snapshot of device firmware information
        # TYPE redfish_device_info gauge
        redfish_device_info{biosVersion="U99 v0.00 (xx/xx/xxxx)",chassisModel="model a",chassisSerialNumber="SN98765",firmwareVersion="iLO 5 v2.65",name="hostname123"} 1
	`
	GoodCPUStatusExpected = `
        # HELP redfish_cpu_status Current cpu status 1 = OK, 0 = BAD
        # TYPE redfish_cpu_status gauge
        redfish_cpu_status{chassisModel="model a",chassisSerialNumber="SN98765",id="1",model="cpu model",socket="Proc 1",totalCores="99"} 1
	`
	GoodLogicalDriveExpected = `
        # HELP redfish_logical_drive_status Current Logical Drive Raid 1 = OK, 0 = BAD, -1 = DISABLED
        # TYPE redfish_logical_drive_status gauge
        redfish_logical_drive_status{chassisModel="model a",chassisSerialNumber="SN98765",logicaldrivename="TESTDRIVE NAME 1",name="HpeSmartStorageLogicalDrive",raid="1",volumeuniqueidentifier="ABCDEF12345"} 1
	`
	GoodDiskDriveExpected = `
        # HELP redfish_disk_drive_status Current Disk Drive status 1 = OK, 0 = BAD, -1 = DISABLED
        # TYPE redfish_disk_drive_status gauge
        redfish_disk_drive_status{capacityMiB="915715",chassisModel="model a",chassisSerialNumber="SN98765",id="0",location="1I:1:1",name="HpeSmartStorageDiskDrive",serialnumber="ABC123"} 1
	`
	GoodNvmeDriveExpected = `
        # HELP redfish_nvme_drive_status Current NVME status 1 = OK, 0 = BAD, -1 = DISABLED
        # TYPE redfish_nvme_drive_status gauge
        redfish_nvme_drive_status{chassisModel="model a",chassisSerialNumber="SN98765",id="DA000000",protocol="NVMe",serviceLabel="Box 3:Bay 7"} 1
	`
	GoodStorageControllerExpected = `
        # HELP redfish_storage_controller_status Current storage controller status 1 = OK, 0 = BAD
        # TYPE redfish_storage_controller_status gauge
        redfish_storage_controller_status{chassisModel="model a",chassisSerialNumber="SN98765",firmwareVersion="x.xxx.xx-xxxx",location="",model="raid model",name="SBMezz1"} 1
	`
	GoodILOSelfTestExpected = `
        # HELP redfish_ilo_selftest_status Current ilo selftest status 1 = OK, 0 = BAD
        # TYPE redfish_ilo_selftest_status gauge
        redfish_ilo_selftest_status{chassisModel="model a",chassisSerialNumber="SN98765",name="EEPROM"} 1
	`
	GoodStorageBatteryStatusExpected = `
        # HELP redfish_storage_battery_status Current storage battery status 1 = OK, 0 = BAD
        # TYPE redfish_storage_battery_status gauge
        redfish_storage_battery_status{chassisModel="model a",chassisSerialNumber="SN98765",id="1",model="battery model",name="HPE Smart Storage Battery"} 1
	`
	GoodMemoryDimmExpected = `
        # HELP redfish_memory_dimm_status Current dimm status 1 = OK, 0 = BAD
        # TYPE redfish_memory_dimm_status gauge
        redfish_memory_dimm_status{capacityMiB="32768",chassisModel="model a",chassisSerialNumber="SN98765",manufacturer="HPE",name="proc1dimm1",partNumber="part number"} 1
	`
	GoodMemoryDimmExpectedG9 = `
        # HELP redfish_memory_dimm_status Current dimm status 1 = OK, 0 = BAD
        # TYPE redfish_memory_dimm_status gauge
        redfish_memory_dimm_status{capacityMiB="32768",chassisModel="model a",chassisSerialNumber="SN98765",manufacturer="HP",name="proc2dimm12",partNumber="part number"} 1
	`
	GoodMemorySummaryExpected = `
        # HELP redfish_memory_status Current memory status 1 = OK, 0 = BAD
        # TYPE redfish_memory_status gauge
        redfish_memory_status{chassisModel="model a",chassisSerialNumber="SN98765",totalSystemMemoryGiB="384"} 1
	`
	GoodThermalSummaryExpected = `
        # HELP redfish_thermal_summary_status Current sensor status 1 = OK, 0 = BAD
        # TYPE redfish_thermal_summary_status gauge
        redfish_thermal_summary_status{chassisModel="model a",chassisSerialNumber="SN98765",url="/redfish/v1/Chassis/1/Thermal/"} 1
	`
	GoodThermalFanSpeedExpected = `
        # HELP redfish_thermal_fan_speed Current fan speed in the unit of percentage, possible values are 0 - 100
        # TYPE redfish_thermal_fan_speed gauge
        redfish_thermal_fan_speed{chassisModel="model a",chassisSerialNumber="SN98765",name="Fan 1"} 16
	`
	GoodThermalFanStatusExpected = `
        # HELP redfish_thermal_fan_status Current fan status 1 = OK, 0 = BAD
        # TYPE redfish_thermal_fan_status gauge
        redfish_thermal_fan_status{chassisModel="model a",chassisSerialNumber="SN98765",name="Fan 1"} 1
	`
	GoodThermalSensorStatusExpected = `
        # HELP redfish_thermal_sensor_status Current sensor status 1 = OK, 0 = BAD
        # TYPE redfish_thermal_sensor_status gauge
        redfish_thermal_sensor_status{chassisModel="model a",chassisSerialNumber="SN98765",name="01-Inlet Ambient"} 1
	`
	GoodThermalSensorTempExpected = `
        # HELP redfish_thermal_sensor_temperature Current sensor temperature reading in Celsius
        # TYPE redfish_thermal_sensor_temperature gauge
        redfish_thermal_sensor_temperature{chassisModel="model a",chassisSerialNumber="SN98765",name="01-Inlet Ambient"} 22
	`
	GoodPowerVoltageOutputExpected = `
        # HELP redfish_power_voltage_output Power voltage output in watts
        # TYPE redfish_power_voltage_output gauge
        redfish_power_voltage_output{chassisModel="model a",chassisSerialNumber="SN98765",name="PSU1_VOUT"} 12.2
	`
	GoodPowerVoltageStatusExpected = `
        # HELP redfish_power_voltage_status Current power voltage status 1 = OK, 0 = BAD
        # TYPE redfish_power_voltage_status gauge
        redfish_power_voltage_status{chassisModel="model a",chassisSerialNumber="SN98765",name="PSU1_VOUT"} 1
	`
	GoodPowerSupplyOutputOemHpeExpected = `
        # HELP redfish_power_supply_output Power supply output in watts
        # TYPE redfish_power_supply_output gauge
        redfish_power_supply_output{bayNumber="1",chassisModel="model a",chassisSerialNumber="SN98765",firmwareVersion="x.xx",manufacturer="DELTA",model="psmodel",name="HpeServerPowerSupply",powerSupplyType="AC",serialNumber="999999"} 91
	`
	GoodPowerSupplyStatusOemHpeExpected = `
        # HELP redfish_power_supply_status Current power supply status 1 = OK, 0 = BAD
        # TYPE redfish_power_supply_status gauge
        redfish_power_supply_status{bayNumber="1",chassisModel="model a",chassisSerialNumber="SN98765",firmwareVersion="x.xx",manufacturer="DELTA",model="psmodel",name="HpeServerPowerSupply",powerSupplyType="AC",serialNumber="999999"} 1
	`
	GoodPowerSupplyOutputOemHpExpected = `
        # HELP redfish_power_supply_output Power supply output in watts
        # TYPE redfish_power_supply_output gauge
        redfish_power_supply_output{bayNumber="2",chassisModel="model a",chassisSerialNumber="SN98765",firmwareVersion="x.xx",manufacturer="DELTA",model="psmodel",name="HpeServerPowerSupply",powerSupplyType="AC",serialNumber="999999"} 91
	`
	GoodPowerSupplyStatusOemHpExpected = `
        # HELP redfish_power_supply_status Current power supply status 1 = OK, 0 = BAD
        # TYPE redfish_power_supply_status gauge
        redfish_power_supply_status{bayNumber="2",chassisModel="model a",chassisSerialNumber="SN98765",firmwareVersion="x.xx",manufacturer="DELTA",model="psmodel",name="HpeServerPowerSupply",powerSupplyType="AC",serialNumber="999999"} 1
	`
	GoodPowerSupplyTotalConsumedExpected = `
        # HELP redfish_power_supply_total_consumed Total output of all power supplies in watts
        # TYPE redfish_power_supply_total_consumed gauge
        redfish_power_supply_total_consumed{chassisModel="model a",chassisSerialNumber="SN98765",memberId="/redfish/v1/Chassis/1/Power"} 206
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

func Test_Exporter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redfish/v1/badcred/Chassis/" {
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
			var mockExcludes = make(map[string]interface{})
			exporter, err = NewExporter(ctx, server.URL, test.uri, "", "model a", mockExcludes)
			assert.Nil(err)
			assert.NotNil(exporter)

			prometheus.MustRegister(exporter)

			metric := (*exporter.(*Exporter).DeviceMetrics)[test.metricRef1]
			m := (*metric)[test.metricRef2]

			assert.Empty(testutil.CollectAndCompare(m, strings.NewReader(test.expected), test.metricName))

			prometheus.Unregister(exporter)

		})
	}
}

// Test_Exporter_Upper_Lower_Links tests the uppercase and lowercase Links/links struct because of
// the different firmware versions of the redfish API
func Test_Exporter_Upper_Lower_Links(t *testing.T) {
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

func Test_Exporter_Metrics_Handling(t *testing.T) {

	var GoodDeviceInfoResponse = []byte(`{
			"Description": "test description",
  			"FirmwareVersion": "iLO 5 v2.65",
       	    "SerialNumber": "SN99999"
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
	var GoodStorageControllerResponse = []byte(`{
  			"Name": "SBMezz1",
       		"StorageControllers": [
              {
                "Name": "raid name",
      		    "Status": {
      	          "State": "Enabled",
      	          "HealthRollup": "OK",
      	          "Health": "OK"
                },
                "MemberId": "RAID",
                "Manufacturer": "LSI Logic",
                "FirmwareVersion": "x.xxx.xx-xxxx",
                "SupportedControllerProtocols": [
               	  "PCIe"
                ],
                "SerialNumber": "SN98765",
                "Model": "raid model",
                "SupportedDeviceProtocols": [
               	  "SAS"
       	        ],
       	        "CacheSummary": {
      	          "TotalCacheSizeMiB": 3087
                }
              }
            ]
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
	var GoodThermalSummaryResponse = []byte(`{
  			"@odata.id": "/redfish/v1/Chassis/1/Thermal/",
     		"Id": "Thermal",
       		"Name": "Thermal",
       		"Status": {
           		"State": "Enabled",
             	"Health": "OK"
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
	var GoodPowerSupplyOutputOemHpeResponse = []byte(`{
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
  			    "FirmwareVersion": "x.xx",
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
                "SerialNumber": "999999",
  			    "SparePartNumber": "part number",
  			    "Status": {
  			      "Health": "OK",
  			      "State": "Enabled"
  			    }
  			  }
  			]
  		}`)
	var GoodPowerSupplyStatusOemHpeResponse = []byte(`{
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
  			    "FirmwareVersion": "x.xx",
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
                "SerialNumber": "999999",
  			    "SparePartNumber": "part number",
  			    "Status": {
  			      "Health": "OK",
  			      "State": "Enabled"
  			    }
  			  }
  			]
  		}`)
	var GoodPowerSupplyOutputOemHpResponse = []byte(`{
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
  			    "FirmwareVersion": "x.xx",
  			    "LastPowerOutputWatts": 91,
  			    "LineInputVoltage": 206,
  			    "LineInputVoltageType": "ACHighLine",
  			    "Manufacturer": "DELTA",
  			    "MemberId": "0",
  			    "Model": "psmodel",
  			    "Name": "HpeServerPowerSupply",
  			    "Oem": {
  			      "Hp": {
  			        "AveragePowerOutputWatts": 91,
  			        "BayNumber": 2,
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
                "SerialNumber": "999999",
  			    "SparePartNumber": "part number",
  			    "Status": {
  			      "Health": "OK",
  			      "State": "Enabled"
  			    }
  			  }
  			]
  		}`)
	var GoodPowerSupplyStatusOemHpResponse = []byte(`{
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
  			    "FirmwareVersion": "x.xx",
  			    "LastPowerOutputWatts": 91,
  			    "LineInputVoltage": 206,
  			    "LineInputVoltageType": "ACHighLine",
  			    "Manufacturer": "DELTA",
  			    "MemberId": "0",
  			    "Model": "psmodel",
  			    "Name": "HpeServerPowerSupply",
  			    "Oem": {
  			      "Hp": {
  			        "AveragePowerOutputWatts": 91,
  			        "BayNumber": 2,
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
                "SerialNumber": "999999",
  			    "SparePartNumber": "part number",
  			    "Status": {
  			      "Health": "OK",
  			      "State": "Enabled"
  			    }
  			  }
  			]
  		}`)
	var GoodPowerSupplyTotalConsumedResponse = []byte(`{
			"@odata.id": "/redfish/v1/Chassis/1/Power",
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

	exporter = &Exporter{
		ctx:                 context.Background(),
		host:                "fishymetrics.com",
		Model:               "model a",
		biosVersion:         "U99 v0.00 (xx/xx/xxxx)",
		systemHostname:      "hostname123",
		ChassisSerialNumber: "SN98765",
		DeviceMetrics:       NewDeviceMetrics(),
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

	storCtrlMetrics := func(exp *Exporter, payload []byte) error {
		err := exp.exportStorageControllerMetrics(payload)
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
			metricName: "redfish_device_info",
			metricRef1: "deviceInfo",
			metricRef2: "deviceInfo",
			handleFunc: deviceInfoMetrics,
			response:   GoodDeviceInfoResponse,
			expected:   GoodDeviceInfoExpected,
		},
		{
			name:       "Good CPU Status",
			metricName: "redfish_cpu_status",
			metricRef1: "processorMetrics",
			metricRef2: "processorStatus",
			handleFunc: processorMetrics,
			response:   GoodCPUStatusResponse,
			expected:   GoodCPUStatusExpected,
		},
		{
			name:       "Good Logical Drive",
			metricName: "redfish_logical_drive_status",
			metricRef1: "logicalDriveMetrics",
			metricRef2: "raidStatus",
			handleFunc: logicalDevMetrics,
			response:   GoodLogicalDriveResponse,
			expected:   GoodLogicalDriveExpected,
		},
		{
			name:       "Good Disk Drive",
			metricName: "redfish_disk_drive_status",
			metricRef1: "diskDriveMetrics",
			metricRef2: "driveStatus",
			handleFunc: physDevMetrics,
			response:   GoodDiskDriveResponse,
			expected:   GoodDiskDriveExpected,
		},
		{
			name:       "Good Nvme Drive",
			metricName: "redfish_nvme_drive_status",
			metricRef1: "nvmeMetrics",
			metricRef2: "nvmeDriveStatus",
			handleFunc: nvmeDevMetrics,
			response:   GoodNvmeDriveResponse,
			expected:   GoodNvmeDriveExpected,
		},
		{
			name:       "Good Storage Controller",
			metricName: "redfish_storage_controller_status",
			metricRef1: "storageCtrlMetrics",
			metricRef2: "storageControllerStatus",
			handleFunc: storCtrlMetrics,
			response:   GoodStorageControllerResponse,
			expected:   GoodStorageControllerExpected,
		},
		{
			name:       "Good iLO Self Test",
			metricName: "redfish_ilo_selftest_status",
			metricRef1: "iloSelfTestMetrics",
			metricRef2: "iloSelfTestStatus",
			handleFunc: iloSelfTestMetrics,
			response:   GoodILOSelfTestResponse,
			expected:   GoodILOSelfTestExpected,
		},
		{
			name:       "Good Storage Battery Status",
			metricName: "redfish_storage_battery_status",
			metricRef1: "storBatteryMetrics",
			metricRef2: "storageBatteryStatus",
			handleFunc: storBatterytMetrics,
			response:   GoodStorageBatteryStatusResponse,
			expected:   GoodStorageBatteryStatusExpected,
		},
		{
			name:       "Good Memory DIMM Status",
			metricName: "redfish_memory_dimm_status",
			metricRef1: "memoryMetrics",
			metricRef2: "memoryDimmStatus",
			handleFunc: memDimmMetrics,
			response:   GoodMemoryDimmResponse,
			expected:   GoodMemoryDimmExpected,
		},
		{
			name:       "Good Memory DIMM Status G9",
			metricName: "redfish_memory_dimm_status",
			metricRef1: "memoryMetrics",
			metricRef2: "memoryDimmStatus",
			handleFunc: memDimmMetrics,
			response:   GoodMemoryDimmResponseG9,
			expected:   GoodMemoryDimmExpectedG9,
		},
		{
			name:       "Good Memory Summary Status",
			metricName: "redfish_memory_status",
			metricRef1: "memoryMetrics",
			metricRef2: "memoryStatus",
			handleFunc: memSummaryMetrics,
			response:   GoodMemorySummaryResponse,
			expected:   GoodMemorySummaryExpected,
		},
		{
			name:       "Good Thermal Summary Status",
			metricName: "redfish_thermal_summary_status",
			metricRef1: "thermalMetrics",
			metricRef2: "thermalSummary",
			handleFunc: thermMetrics,
			response:   GoodThermalSummaryResponse,
			expected:   GoodThermalSummaryExpected,
		},
		{
			name:       "Good Thermal Fan Speed",
			metricName: "redfish_thermal_fan_speed",
			metricRef1: "thermalMetrics",
			metricRef2: "fanSpeed",
			handleFunc: thermMetrics,
			response:   GoodThermalFanSpeedResponse,
			expected:   GoodThermalFanSpeedExpected,
		},
		{
			name:       "Good Thermal Fan Status",
			metricName: "redfish_thermal_fan_status",
			metricRef1: "thermalMetrics",
			metricRef2: "fanStatus",
			handleFunc: thermMetrics,
			response:   GoodThermalFanStatusResponse,
			expected:   GoodThermalFanStatusExpected,
		},
		{
			name:       "Good Thermal Sensor Status",
			metricName: "redfish_thermal_sensor_status",
			metricRef1: "thermalMetrics",
			metricRef2: "sensorStatus",
			handleFunc: thermMetrics,
			response:   GoodThermalSensorStatusResponse,
			expected:   GoodThermalSensorStatusExpected,
		},
		{
			name:       "Good Thermal Sensor Temperature",
			metricName: "redfish_thermal_sensor_temperature",
			metricRef1: "thermalMetrics",
			metricRef2: "sensorTemperature",
			handleFunc: thermMetrics,
			response:   GoodThermalSensorTempResponse,
			expected:   GoodThermalSensorTempExpected,
		},
		{
			name:       "Good Power Voltage Output",
			metricName: "redfish_power_voltage_output",
			metricRef1: "powerMetrics",
			metricRef2: "voltageOutput",
			handleFunc: powMetrics,
			response:   GoodPowerVoltageOutputResponse,
			expected:   GoodPowerVoltageOutputExpected,
		},
		{
			name:       "Good Power Voltage Status",
			metricName: "redfish_power_voltage_status",
			metricRef1: "powerMetrics",
			metricRef2: "voltageStatus",
			handleFunc: powMetrics,
			response:   GoodPowerVoltageStatusResponse,
			expected:   GoodPowerVoltageStatusExpected,
		},
		{
			name:       "Good Power Supply Output Oem Hpe",
			metricName: "redfish_power_supply_output",
			metricRef1: "powerMetrics",
			metricRef2: "supplyOutput",
			handleFunc: powMetrics,
			response:   GoodPowerSupplyOutputOemHpeResponse,
			expected:   GoodPowerSupplyOutputOemHpeExpected,
		},
		{
			name:       "Good Power Supply Status Oem Hpe",
			metricName: "redfish_power_supply_status",
			metricRef1: "powerMetrics",
			metricRef2: "supplyStatus",
			handleFunc: powMetrics,
			response:   GoodPowerSupplyStatusOemHpeResponse,
			expected:   GoodPowerSupplyStatusOemHpeExpected,
		},
		{
			name:       "Good Power Supply Output Oem Hp",
			metricName: "redfish_power_supply_output",
			metricRef1: "powerMetrics",
			metricRef2: "supplyOutput",
			handleFunc: powMetrics,
			response:   GoodPowerSupplyOutputOemHpResponse,
			expected:   GoodPowerSupplyOutputOemHpExpected,
		},
		{
			name:       "Good Power Supply Status Oem Hp",
			metricName: "redfish_power_supply_status",
			metricRef1: "powerMetrics",
			metricRef2: "supplyStatus",
			handleFunc: powMetrics,
			response:   GoodPowerSupplyStatusOemHpResponse,
			expected:   GoodPowerSupplyStatusOemHpExpected,
		},
		{
			name:       "Good Power Supply Total Consumed",
			metricName: "redfish_power_supply_total_consumed",
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
			metric := (*exporter.(*Exporter).DeviceMetrics)[test.metricRef1]
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
