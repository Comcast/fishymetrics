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

package dl380

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
	GoodLogicalDriveExpected = `
		 # HELP dl380_logical_drive_status Current Logical Drive Raid 1 = OK, 0 = BAD, -1 = DISABLED
		 # TYPE dl380_logical_drive_status gauge
		 dl380_logical_drive_status{chassisSerialNumber="SN98765",logicaldrivename="TESTDRIVE NAME 1",name="HpeSmartStorageLogicalDrive",raid="1",volumeuniqueidentifier="ABCDEF12345"} 1
	`
	GoodDiskDriveExpected = `
		 # HELP dl380_disk_drive_status Current Disk Drive status 1 = OK, 0 = BAD, -1 = DISABLED
		 # TYPE dl380_disk_drive_status gauge
		 dl380_disk_drive_status{chassisSerialNumber="SN98765",id="0",location="1I:1:1",name="HpeSmartStorageDiskDrive",serialnumber="ABC123"} 1
	`
	GoodNvmeDriveExpected = `
		 # HELP dl380_nvme_drive_status Current NVME status 1 = OK, 0 = BAD, -1 = DISABLED
		 # TYPE dl380_nvme_drive_status gauge
		 dl380_nvme_drive_status{chassisSerialNumber="SN98765",id="DA000000",protocol="NVMe",serviceLabel="Box 3:Bay 7"} 1
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

func Test_DL380_Exporter(t *testing.T) {
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

func Test_DL380_Metrics_Handling(t *testing.T) {

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

	var exporter prometheus.Collector

	assert := assert.New(t)

	metrx := NewDeviceMetrics()

	exporter = &Exporter{
		ctx:                 context.Background(),
		host:                "fishymetrics.com",
		biosVersion:         "U99 v0.00 (xx/xx/xxxx)",
		chassisSerialNumber: "SN98765",
		deviceMetrics:       metrx,
	}

	prometheus.MustRegister(exporter)

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
		metricName string
		metricRef1 string
		metricRef2 string
		handleFunc func(*Exporter, []byte) error
		response   []byte
		expected   string
	}{
		{
			name:       "Good Logical Drive",
			metricName: "dl380_logical_drive_status",
			metricRef1: "logicalDriveMetrics",
			metricRef2: "raidStatus",
			handleFunc: logicalDevMetrics,
			response:   GoodLogicalDriveResponse,
			expected:   GoodLogicalDriveExpected,
		},
		{
			name:       "Good Disk Drive",
			metricName: "dl380_disk_drive_status",
			metricRef1: "diskDriveMetrics",
			metricRef2: "driveStatus",
			handleFunc: physDevMetrics,
			response:   GoodDiskDriveResponse,
			expected:   GoodDiskDriveExpected,
		},
		{
			name:       "Good Nvme Drive",
			metricName: "dl380_nvme_drive_status",
			metricRef1: "nvmeMetrics",
			metricRef2: "nvmeDriveStatus",
			handleFunc: nvmeDevMetrics,
			response:   GoodNvmeDriveResponse,
			expected:   GoodNvmeDriveExpected,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
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
