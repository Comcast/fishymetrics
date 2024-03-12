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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

const (
	logicalDrivesUpperResponse = `
		 # HELP dl360_logical_drive_status Current Logical Drive Raid 1 = OK, 0 = BAD, -1 = DISABLED
		 # TYPE dl360_logical_drive_status gauge
		 dl360_logical_drive_status{logicaldrivename="TESTDRIVE NAME 1",name="HpeSmartStorageLogicalDrive",raid="1",volumeuniqueidentifier="ABCDEF12345"} 1
	`
	diskDrivesUpperResponse = `
		 # HELP dl360_disk_drive_status Current Disk Drive status 1 = OK, 0 = BAD, -1 = DISABLED
		 # TYPE dl360_disk_drive_status gauge
		 dl360_disk_drive_status{id="0",location="1I:1:1",name="HpeSmartStorageDiskDrive",serialnumber="ABC123"} 1
	`
	logicalDrivesLowerResponse = `
		 # HELP dl360_logical_drive_status Current Logical Drive Raid 1 = OK, 0 = BAD, -1 = DISABLED
		 # TYPE dl360_logical_drive_status gauge
		 dl360_logical_drive_status{logicaldrivename="TESTDRIVE NAME 2",name="HpeSmartStorageLogicalDrive",raid="1",volumeuniqueidentifier="FEDCBA12345"} 1
	`
	diskDrivesLowerResponse = `
		 # HELP dl360_disk_drive_status Current Disk Drive status 1 = OK, 0 = BAD, -1 = DISABLED
		 # TYPE dl360_disk_drive_status gauge
		 dl360_disk_drive_status{id="1",location="1I:1:2",name="HpeSmartStorageDiskDrive",serialnumber="DEF456"} 1
	`
)

var (
	GoodUpperDiskDrive = MustMarshal(DiskDriveMetrics{
		Id:            "0",
		CapacityMiB:   572325,
		Description:   "HPE Smart Storage Disk Drive View",
		InterfaceType: "SAS",
		Name:          "HpeSmartStorageDiskDrive",
		Model:         "TESTMODEL",
		Status: DriveStatus{
			Health: "OK",
			State:  "Enabled",
		},
		Location:     "1I:1:1",
		SerialNumber: "ABC123",
	})
	GoodLowerDiskDrive = MustMarshal(DiskDriveMetrics{
		Id:            "1",
		CapacityMiB:   572325,
		Description:   "HPE Smart Storage Disk Drive View",
		InterfaceType: "SAS",
		Name:          "HpeSmartStorageDiskDrive",
		Model:         "TESTMODEL",
		Status: DriveStatus{
			Health: "OK",
			State:  "Enabled",
		},
		Location:     "1I:1:2",
		SerialNumber: "DEF456",
	})
	GoodUpperLogicalDrive = MustMarshal(LogicalDriveMetrics{
		Id:                 "1",
		CapacityMiB:        572293,
		Description:        "HPE Smart Storage Logical Drive View",
		InterfaceType:      "SAS",
		LogicalDriveName:   "TESTDRIVE NAME 1",
		LogicalDriveNumber: 1,
		Name:               "HpeSmartStorageLogicalDrive",
		Raid:               "1",
		Status: DriveStatus{
			Health: "OK",
			State:  "Enabled",
		},
		StripeSizebytes:        262144,
		VolumeUniqueIdentifier: "ABCDEF12345",
	})
	GoodLowerLogicalDrive = MustMarshal(LogicalDriveMetrics{
		Id:                 "1",
		CapacityMiB:        572293,
		Description:        "HPE Smart Storage Logical Drive View",
		InterfaceType:      "SAS",
		LogicalDriveName:   "TESTDRIVE NAME 2",
		LogicalDriveNumber: 1,
		Name:               "HpeSmartStorageLogicalDrive",
		Raid:               "1",
		Status: DriveStatus{
			Health: "OK",
			State:  "Enabled",
		},
		StripeSizebytes:        262144,
		VolumeUniqueIdentifier: "FEDCBA12345",
	})
)

func Test_DL360_Drives(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/" {
			w.WriteHeader(http.StatusOK)
			w.Write(MustMarshal(GenericDrive{
				MembersCount: 2,
				Members: []Members{
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
			w.Write(MustMarshal(GenericDrive{
				LinksUpper: LinksUpper{
					LogicalDrives: URL{
						URL: "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/0/LogicalDrives/",
					},
					PhysicalDrives: URL{
						URL: "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/0/DiskDrives/",
					},
				},
			}))
			return
		} else if r.URL.Path == "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/2/" {
			w.WriteHeader(http.StatusOK)
			w.Write(MustMarshal(GenericDrive{
				LinksLower: LinksLower{
					LogicalDrives: HRef{
						URL: "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/2/LogicalDrives/",
					},
					PhysicalDrives: HRef{
						URL: "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/2/DiskDrives/",
					},
				},
			}))
			return
		} else if r.URL.Path == "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/0/LogicalDrives/" {
			w.WriteHeader(http.StatusOK)
			w.Write(MustMarshal(GenericDrive{
				MembersCount: 1,
				Members: []Members{
					{
						URL: "/redfish/v1/Systems/1/SmartStorage/ArrayControllers/0/LogicalDrives/1/",
					},
				},
			}))
			return
		} else if r.URL.Path == "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/2/LogicalDrives/" {
			w.WriteHeader(http.StatusOK)
			w.Write(MustMarshal(GenericDrive{
				MembersCount: 1,
				Members: []Members{
					{
						URL: "/redfish/v1/Systems/1/SmartStorage/ArrayControllers/2/LogicalDrives/1/",
					},
				},
			}))
			return
		} else if r.URL.Path == "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/0/DiskDrives/" {
			w.WriteHeader(http.StatusOK)
			w.Write(MustMarshal(GenericDrive{
				MembersCount: 1,
				Members: []Members{
					{
						URL: "/redfish/v1/Systems/1/SmartStorage/ArrayControllers/0/DiskDrives/0/",
					},
				},
			}))
			return
		} else if r.URL.Path == "/redfish/v1/good/Systems/1/SmartStorage/ArrayControllers/2/DiskDrives/" {
			w.WriteHeader(http.StatusOK)
			w.Write(MustMarshal(GenericDrive{
				MembersCount: 1,
				Members: []Members{
					{
						URL: "/redfish/v1/Systems/1/SmartStorage/ArrayControllers/2/DiskDrives/0/",
					},
				},
			}))
			return
		} else if r.URL.Path == "/redfish/v1/good/Chassis/1/" {
			w.WriteHeader(http.StatusOK)
			w.Write(MustMarshal(GenericDrive{
				LinksUpper: LinksUpper{},
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
			name:       "Member Links Upper Logical Drive",
			uri:        "/redfish/v1/good",
			metricName: "dl360_logical_drive_status",
			metricRef1: "logicalDriveMetrics",
			metricRef2: "raidStatus",
			exportFunc: logicalDevMetrics,
			payload:    GoodUpperLogicalDrive,
			expected:   logicalDrivesUpperResponse,
		},
		{
			name:       "Member Links Lower Logical Drive",
			uri:        "/redfish/v1/good",
			metricName: "dl360_logical_drive_status",
			metricRef1: "logicalDriveMetrics",
			metricRef2: "raidStatus",
			exportFunc: logicalDevMetrics,
			payload:    GoodLowerLogicalDrive,
			expected:   logicalDrivesLowerResponse,
		},
		{
			name:       "Member Links Upper Disk Drive",
			uri:        "/redfish/v1/good",
			metricName: "dl360_disk_drive_status",
			metricRef1: "diskDriveMetrics",
			metricRef2: "driveStatus",
			exportFunc: physDevMetrics,
			payload:    GoodUpperDiskDrive,
			expected:   diskDrivesUpperResponse,
		},
		{
			name:       "Member Links Lower Disk Drive",
			uri:        "/redfish/v1/good",
			metricName: "dl360_disk_drive_status",
			metricRef1: "diskDriveMetrics",
			metricRef2: "driveStatus",
			exportFunc: physDevMetrics,
			payload:    GoodLowerDiskDrive,
			expected:   diskDrivesLowerResponse,
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
