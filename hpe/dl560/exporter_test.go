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

package dl560

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
	up2Response = `
		 # HELP up was the last scrape of fishymetrics successful.
		 # TYPE up gauge
		 up 2
	`
	GoodLogicalDriveUpperResponse = `
		 # HELP dl560_logical_drive_status Current Logical Drive Raid 1 = OK, 0 = BAD, -1 = DISABLED
		 # TYPE dl560_logical_drive_status gauge
		 dl560_logical_drive_status{logicaldrivename="TESTDRIVE NAME 1",name="HpeSmartStorageLogicalDrive",raid="1",volumeuniqueidentifier="ABCDEF12345"} 1
	`
	GoodDiskDriveUpperResponse = `
		 # HELP dl560_disk_drive_status Current Disk Drive status 1 = OK, 0 = BAD, -1 = DISABLED
		 # TYPE dl560_disk_drive_status gauge
		 dl560_disk_drive_status{id="0",location="1I:1:1",name="HpeSmartStorageDiskDrive",serialnumber="ABC123"} 1
	`
	GoodLogicalDriveLowerResponse = `
		 # HELP dl560_logical_drive_status Current Logical Drive Raid 1 = OK, 0 = BAD, -1 = DISABLED
		 # TYPE dl560_logical_drive_status gauge
		 dl560_logical_drive_status{logicaldrivename="TESTDRIVE NAME 2",name="HpeSmartStorageLogicalDrive",raid="1",volumeuniqueidentifier="FEDCBA12345"} 1
	`
	GoodDiskDriveLowerResponse = `
		 # HELP dl560_disk_drive_status Current Disk Drive status 1 = OK, 0 = BAD, -1 = DISABLED
		 # TYPE dl560_disk_drive_status gauge
		 dl560_disk_drive_status{id="1",location="1I:1:2",name="HpeSmartStorageDiskDrive",serialnumber="DEF456"} 1
	`
	GoodNvmeDriveResponse = `
		 # HELP dl560_nvme_drive_status Current NVME status 1 = OK, 0 = BAD, -1 = DISABLED
		 # TYPE dl560_nvme_drive_status gauge
		 dl560_nvme_drive_status{id="0",protocol="NVMe",serviceLabel="Box 3:Bay 7"} 1
	`
)

var (
	GoodDiskDriveUpper = MustMarshal(struct {
		Id            string `json:"Id"`
		CapacityMiB   int    `json:"CapacityMiB"`
		Description   string `json:"Description"`
		InterfaceType string `json:"InterfaceType"`
		Name          string `json:"Name"`
		Model         string `json:"Model"`
		Status        struct {
			Health string `json:"Health"`
			State  string `json:"State"`
		} `json:"Status"`
		Location     string `json:"Location"`
		SerialNumber string `json:"SerialNumber"`
	}{
		Id:            "0",
		CapacityMiB:   572325,
		Description:   "HPE Smart Storage Disk Drive View",
		InterfaceType: "SAS",
		Name:          "HpeSmartStorageDiskDrive",
		Model:         "TESTMODEL",
		Status: struct {
			Health string `json:"Health"`
			State  string `json:"State"`
		}{
			Health: "OK",
			State:  "Enabled",
		},
		Location:     "1I:1:1",
		SerialNumber: "ABC123",
	})

	GoodDiskDriveLower = MustMarshal(struct {
		Id            string `json:"Id"`
		CapacityMiB   int    `json:"CapacityMiB"`
		Description   string `json:"Description"`
		InterfaceType string `json:"InterfaceType"`
		Name          string `json:"Name"`
		Model         string `json:"Model"`
		Status        struct {
			Health string `json:"Health"`
			State  string `json:"State"`
		} `json:"Status"`
		Location     string `json:"Location"`
		SerialNumber string `json:"SerialNumber"`
	}{
		Id:            "1",
		CapacityMiB:   572325,
		Description:   "HPE Smart Storage Disk Drive View",
		InterfaceType: "SAS",
		Name:          "HpeSmartStorageDiskDrive",
		Model:         "TESTMODEL",
		Status: struct {
			Health string `json:"Health"`
			State  string `json:"State"`
		}{
			Health: "OK",
			State:  "Enabled",
		},
		Location:     "1I:1:2",
		SerialNumber: "DEF456",
	})

	GoodLogicalDriveUpper = MustMarshal(struct {
		Id                 string `json:"Id"`
		CapacityMiB        int    `json:"CapacityMiB"`
		Description        string `json:"Description"`
		InterfaceType      string `json:"InterfaceType"`
		LogicalDriveName   string `json:"LogicalDriveName"`
		LogicalDriveNumber int    `json:"LogicalDriveNumber"`
		Name               string `json:"Name"`
		Raid               string `json:"Raid"`
		Status             struct {
			Health string `json:"Health"`
			State  string `json:"State"`
		} `json:"Status"`
		StripeSizebytes        int    `json:"StripeSizebytes"`
		VolumeUniqueIdentifier string `json:"VolumeUniqueIdentifier"`
	}{
		Id:                 "1",
		CapacityMiB:        572293,
		Description:        "HPE Smart Storage Disk Drive View",
		InterfaceType:      "SAS",
		LogicalDriveName:   "TESTDRIVE NAME 1",
		LogicalDriveNumber: 1,
		Name:               "HpeSmartStorageLogicalDrive",
		Raid:               "1",
		Status: struct {
			Health string `json:"Health"`
			State  string `json:"State"`
		}{
			Health: "OK",
			State:  "Enabled",
		},
		StripeSizebytes:        262144,
		VolumeUniqueIdentifier: "ABCDEF12345",
	})

	GoodLogicalDriveLower = MustMarshal(struct {
		Id                 string `json:"Id"`
		CapacityMiB        int    `json:"CapacityMiB"`
		Description        string `json:"Description"`
		InterfaceType      string `json:"InterfaceType"`
		LogicalDriveName   string `json:"LogicalDriveName"`
		LogicalDriveNumber int    `json:"LogicalDriveNumber"`
		Name               string `json:"Name"`
		Raid               string `json:"Raid"`
		Status             struct {
			Health string `json:"Health"`
			State  string `json:"State"`
		} `json:"Status"`
		StripeSizebytes        int    `json:"StripeSizebytes"`
		VolumeUniqueIdentifier string `json:"VolumeUniqueIdentifier"`
	}{
		Id:                 "1",
		CapacityMiB:        572293,
		Description:        "HPE Smart Storage Disk Drive View",
		InterfaceType:      "SAS",
		LogicalDriveName:   "TESTDRIVE NAME 2",
		LogicalDriveNumber: 1,
		Name:               "HpeSmartStorageLogicalDrive",
		Raid:               "1",
		Status: struct {
			Health string `json:"Health"`
			State  string `json:"State"`
		}{
			Health: "OK",
			State:  "Enabled",
		},
		StripeSizebytes:        262144,
		VolumeUniqueIdentifier: "FEDCBA12345",
	})

	GoodNvmeDrive = MustMarshal(struct {
		Id        string `json:"Id"`
		Model     string `json:"Model"`
		Name      string `json:"Name"`
		MediaType string `json:"MediaType"`
		Oem       struct {
			Hpe struct {
				DriveStatus struct {
					Health string `json:"Health"`
					State  string `json:"State"`
				} `json:"DriveStatus"`
			} `json:"Hpe"`
		} `json:"Oem"`
		PhysicalLocation struct {
			PartLocation struct {
				ServiceLabel string `json:"ServiceLabel"`
			} `json:"PartLocation"`
		} `json:"PhysicalLocation"`
		Protocol         string `json:"Protocol"`
		FailurePredicted bool   `json:"FailurePredicted"`
		CapacityBytes    int    `json:"CapacityBytes"`
	}{
		Id:        "0",
		Model:     "TESTMODEL",
		Name:      "TESTNAME",
		MediaType: "SSD",
		Oem: struct {
			Hpe struct {
				DriveStatus struct {
					Health string `json:"Health"`
					State  string `json:"State"`
				} `json:"DriveStatus"`
			} `json:"Hpe"`
		}{
			Hpe: struct {
				DriveStatus struct {
					Health string `json:"Health"`
					State  string `json:"State"`
				} `json:"DriveStatus"`
			}{
				DriveStatus: struct {
					Health string `json:"Health"`
					State  string `json:"State"`
				}{
					Health: "OK",
					State:  "Enabled",
				},
			},
		},
		PhysicalLocation: struct {
			PartLocation struct {
				ServiceLabel string `json:"ServiceLabel"`
			} `json:"PartLocation"`
		}{
			PartLocation: struct {
				ServiceLabel string `json:"ServiceLabel"`
			}{
				ServiceLabel: "Box 3:Bay 7",
			},
		},
		Protocol:         "NVMe",
		FailurePredicted: false,
		CapacityBytes:    1600321314816,
	})
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

func Test_DL560_Exporter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redfish/v1/badcred/Systems/1/SmartStorage/ArrayControllers/" {
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
			expected:   up2Response,
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

func Test_DL560_Drives(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/" {
			w.WriteHeader(http.StatusOK)
			w.Write(MustMarshal(struct {
				MembersCount int `json:"Members@odata.count"`
				Members      []struct {
					URL string `json:"@odata.id"`
				} `json:"Members"`
			}{
				MembersCount: 2,
				Members: []struct {
					URL string `json:"@odata.id"`
				}{
					{
						URL: "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/0/",
					},
					{
						URL: "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/2/",
					},
				},
			}))
			return
		} else if r.URL.Path == "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/0/" {
			w.WriteHeader(http.StatusOK)
			w.Write(MustMarshal(struct {
				LinksUpper struct {
					LogicalDrives struct {
						URL string `json:"@odata.id"`
					} `json:"LogicalDrives"`
					PhysicalDrives struct {
						URL string `json:"@odata.id"`
					} `json:"PhysicalDrives"`
				} `json:"Links"`
			}{
				LinksUpper: struct {
					LogicalDrives struct {
						URL string `json:"@odata.id"`
					} `json:"LogicalDrives"`
					PhysicalDrives struct {
						URL string `json:"@odata.id"`
					} `json:"PhysicalDrives"`
				}{
					LogicalDrives: struct {
						URL string `json:"@odata.id"`
					}{
						URL: "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/0/LogicalDrives/",
					},
					PhysicalDrives: struct {
						URL string `json:"@odata.id"`
					}{
						URL: "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/0/DiskDrives/",
					},
				},
			}))
			return
		} else if r.URL.Path == "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/2/" {
			w.WriteHeader(http.StatusOK)
			w.Write(MustMarshal(struct {
				LinksLower struct {
					LogicalDrives struct {
						URL string `json:"href"`
					} `json:"LogicalDrives"`
					PhysicalDrives struct {
						URL string `json:"href"`
					} `json:"PhysicalDrives"`
				} `json:"links"`
			}{
				LinksLower: struct {
					LogicalDrives struct {
						URL string `json:"href"`
					} `json:"LogicalDrives"`
					PhysicalDrives struct {
						URL string `json:"href"`
					} `json:"PhysicalDrives"`
				}{
					LogicalDrives: struct {
						URL string `json:"href"`
					}{
						URL: "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/2/LogicalDrives/",
					},
					PhysicalDrives: struct {
						URL string `json:"href"`
					}{
						URL: "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/2/DiskDrives/",
					},
				},
			}))
			return
		} else if r.URL.Path == "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/0/LogicalDrives/" {
			w.WriteHeader(http.StatusOK)
			w.Write(MustMarshal(struct {
				MembersCount int `json:"Members@odata.count"`
				Members      []struct {
					URL string `json:"@odata.id"`
				} `json:"Members"`
			}{
				MembersCount: 1,
				Members: []struct {
					URL string `json:"@odata.id"`
				}{
					{
						URL: "/redfish/v1/Systems/1/SmartStorage/ArrayControllers/0/LogicalDrives/1/",
					},
				},
			}))
			return
		} else if r.URL.Path == "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/2/LogicalDrives/" {
			w.WriteHeader(http.StatusOK)
			w.Write(MustMarshal(struct {
				MembersCount int `json:"Members@odata.count"`
				Members      []struct {
					URL string `json:"@odata.id"`
				} `json:"Members"`
			}{
				MembersCount: 1,
				Members: []struct {
					URL string `json:"@odata.id"`
				}{
					{
						URL: "/redfish/v1/Systems/1/SmartStorage/ArrayControllers/2/LogicalDrives/1/",
					},
				},
			}))
			return
		} else if r.URL.Path == "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/0/DiskDrives/" {
			w.WriteHeader(http.StatusOK)
			w.Write(MustMarshal(struct {
				MembersCount int `json:"Members@odata.count"`
				Members      []struct {
					URL string `json:"@odata.id"`
				} `json:"Members"`
			}{
				MembersCount: 1,
				Members: []struct {
					URL string `json:"@odata.id"`
				}{
					{
						URL: "/redfish/v1/Systems/1/SmartStorage/ArrayControllers/0/DiskDrives/0/",
					},
				},
			}))
			return
		} else if r.URL.Path == "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/2/DiskDrives/" {
			w.WriteHeader(http.StatusOK)
			w.Write(MustMarshal(struct {
				MembersCount int `json:"Members@odata.count"`
				Members      []struct {
					URL string `json:"@odata.id"`
				} `json:"Members"`
			}{
				MembersCount: 1,
				Members: []struct {
					URL string `json:"@odata.id"`
				}{
					{
						URL: "/redfish/v1/Systems/1/SmartStorage/ArrayControllers/2/DiskDrives/0/",
					},
				},
			}))
			return
		} else if r.URL.Path == "/redfish/v1/good/Chassis/1/" {
			w.WriteHeader(http.StatusOK)
			w.Write(MustMarshal(struct {
				LinksUpper struct {
					Drives []struct {
						URL string `json:"@odata.id"`
					} `json:"Drives"`
				} `json:"Links"`
			}{
				LinksUpper: struct {
					Drives []struct {
						URL string `json:"@odata.id"`
					} `json:"Drives"`
				}{
					Drives: []struct {
						URL string `json:"@odata.id"`
					}{
						{
							URL: "/redfish/v1/Systems/1/Storage/DA000000/Drives/DA000000/",
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

	tests := []struct {
		name       string
		uri        string
		metricName string
		metricRef1 string
		metricRef2 string
		exportFunc func(*Exporter, []byte) error
		payload    []byte
		expected   string
	}{
		{
			name:       "Good Logical Drive Links Uppercase",
			uri:        "/redfish/v1/good",
			metricName: "dl560_logical_drive_status",
			metricRef1: "logicalDriveMetrics",
			metricRef2: "raidStatus",
			exportFunc: logicalDevMetrics,
			payload:    GoodLogicalDriveUpper,
			expected:   GoodLogicalDriveUpperResponse,
		},
		{
			name:       "Good Logical Drive Links Lowercase",
			uri:        "/redfish/v1/good",
			metricName: "dl560_logical_drive_status",
			metricRef1: "logicalDriveMetrics",
			metricRef2: "raidStatus",
			exportFunc: logicalDevMetrics,
			payload:    GoodLogicalDriveLower,
			expected:   GoodLogicalDriveLowerResponse,
		},
		{
			name:       "Good Disk Drive Links Uppercase",
			uri:        "/redfish/v1/good",
			metricName: "dl560_disk_drive_status",
			metricRef1: "diskDriveMetrics",
			metricRef2: "driveStatus",
			exportFunc: physDevMetrics,
			payload:    GoodDiskDriveUpper,
			expected:   GoodDiskDriveUpperResponse,
		},
		{
			name:       "Good Disk Drive Links Lowercase",
			uri:        "/redfish/v1/good",
			metricName: "dl560_disk_drive_status",
			metricRef1: "diskDriveMetrics",
			metricRef2: "driveStatus",
			exportFunc: physDevMetrics,
			payload:    GoodDiskDriveLower,
			expected:   GoodDiskDriveLowerResponse,
		},
		{
			name:       "Good Nvme Drive",
			uri:        "/redfish/v1/good",
			metricName: "dl560_nvme_drive_status",
			metricRef1: "nvmeMetrics",
			metricRef2: "nvmeDriveStatus",
			exportFunc: nvmeDevMetrics,
			payload:    GoodNvmeDrive,
			expected:   GoodNvmeDriveResponse,
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

			err = test.exportFunc(exporter.(*Exporter), test.payload)
			if err != nil {
				t.Error(err)
			}

			metric := (*exporter.(*Exporter).deviceMetrics)[test.metricRef1]
			m := (*metric)[test.metricRef2]

			assert.Empty(testutil.CollectAndCompare(m, strings.NewReader(test.expected), test.metricName))

			prometheus.Unregister(exporter)

		})
	}
}
