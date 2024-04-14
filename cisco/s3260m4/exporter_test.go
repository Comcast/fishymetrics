// /*
//  * Copyright 2024 Comcast Cable Communications Management, LLC
//  *
//  * Licensed under the Apache License, Version 2.0 (the "License");
//  * you may not use this file except in compliance with the License.
//  * You may obtain a copy of the License at
//  *
//  *     http://www.apache.org/licenses/LICENSE-2.0
//  *
//  * Unless required by applicable law or agreed to in writing, software
//  * distributed under the License is distributed on an "AS IS" BASIS,
//  * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  * See the License for the specific language governing permissions and
//  * limitations under the License.
//  */

package s3260m4

// import (
// 	"context"
// 	"encoding/json"
// 	"net/http"
// 	"net/http/httptest"
// 	"strings"
// 	"testing"

// 	"github.com/prometheus/client_golang/prometheus"
// 	"github.com/prometheus/client_golang/prometheus/testutil"
// 	"github.com/stretchr/testify/assert"
// )

// const (
// 	up2Response = `
// 		 # HELP up was the last scrape of fishymetrics successful.
// 		 # TYPE up gauge
// 		 up 2
// 	`
// )

// type TestErrorResponse struct {
// 	Error TestError `json:"error"`
// }

// type TestError struct {
// 	Code         string        `json:"code"`
// 	Message      string        `json:"message"`
// 	ExtendedInfo []TestMessage `json:"@Message.ExtendedInfo"`
// }

// type TestMessage struct {
// 	MessageId string `json:"MessageId"`
// }

// func MustMarshal(v interface{}) []byte {
// 	b, err := json.Marshal(v)
// 	if err != nil {
// 		panic(err)
// 	}
// 	return b
// }

// func Test_S3260M4_Exporter(t *testing.T) {
// 	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		if r.URL.Path == "/redfish/v1/badcred/Managers/BMC2" {
// 			w.WriteHeader(http.StatusUnauthorized)
// 			w.Write(MustMarshal(TestErrorResponse{
// 				Error: TestError{
// 					Code:    "iLO.0.10.ExtendedInfo",
// 					Message: "See @Message.ExtendedInfo for more information.",
// 					ExtendedInfo: []TestMessage{
// 						{
// 							MessageId: "Base.1.0.NoValidSession",
// 						},
// 					},
// 				},
// 			}))
// 			return
// 		}
// 		w.WriteHeader(http.StatusInternalServerError)
// 		w.Write([]byte("Unknown path - please create test case(s) for it"))
// 	}))
// 	defer server.Close()

// 	ctx := context.Background()
// 	assert := assert.New(t)

// 	tests := []struct {
// 		name       string
// 		uri        string
// 		metricName string
// 		metricRef1 string
// 		metricRef2 string
// 		payload    []byte
// 		expected   string
// 	}{
// 		{
// 			name:       "Bad Credentials",
// 			uri:        "/redfish/v1/badcred",
// 			metricName: "up",
// 			metricRef1: "up",
// 			metricRef2: "up",
// 			expected:   up2Response,
// 		},
// 	}

// 	for _, test := range tests {
// 		t.Run(test.name, func(t *testing.T) {
// 			var exporter prometheus.Collector
// 			var err error
// 			exporter, err = NewExporter(ctx, server.URL, test.uri, "")
// 			assert.Nil(err)
// 			assert.NotNil(exporter)

// 			prometheus.MustRegister(exporter)

// 			metric := (*exporter.(*Exporter).deviceMetrics)[test.metricRef1]
// 			m := (*metric)[test.metricRef2]

// 			assert.Empty(testutil.CollectAndCompare(m, strings.NewReader(test.expected), test.metricName))

// 			prometheus.Unregister(exporter)

// 		})
// 	}
// }
