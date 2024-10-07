/*
 * Copyright 2023 Comcast Cable Communications Management, LLC
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

package logger

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/comcast/fishymetrics/config"
	"github.com/hashicorp/go-retryablehttp"
	"go.uber.org/zap"
)

var vectorSinks = map[string]vectorSink{}

type vectorSink struct {
	name     string
	client   *retryablehttp.Client
	endpoint *url.URL
}

func newVectorSink(u *url.URL) vectorSink {

	tr := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 3 * time.Second,
		}).Dial,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.GetConfig().SSLVerify,
		},
		TLSHandshakeTimeout: 10 * time.Second,
	}

	retryClient := retryablehttp.NewClient()
	retryClient.CheckRetry = retryablehttp.ErrorPropagatedRetryPolicy
	retryClient.HTTPClient.Transport = tr
	retryClient.HTTPClient.Timeout = 30 * time.Second
	retryClient.Logger = nil
	retryClient.RetryWaitMin = 1 * time.Second
	retryClient.RetryWaitMax = 1 * time.Second
	retryClient.RetryMax = 2
	retryClient.RequestLogHook = func(l retryablehttp.Logger, r *http.Request, i int) {
		retryCount := i
		if retryCount > 0 {
			fmt.Printf("api call %s failed, retry #%d\n", r.URL.String(), retryCount)
		}
	}
	return vectorSink{
		name:     "vector-sink",
		client:   retryClient,
		endpoint: u,
	}
}

func initVectorSink(u *url.URL) (zap.Sink, error) {
	vectorSinks["vector"] = newVectorSink(u)
	return vectorSinks["vector"], nil
}

// Close implement zap.Sink func Close
func (v vectorSink) Close() error {
	return nil
}

// Write implement zap.Sink func Write
func (v vectorSink) Write(b []byte) (n int, err error) {
	req, _ := retryablehttp.NewRequest(http.MethodPost, v.endpoint.String(), b)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "fishymetrics-vector-http")

	resp, err := v.client.Do(req)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode != http.StatusOK {
		return 0, err
	}

	return len(b), nil
}

// Sync implement zap.Sink func Sync
func (v vectorSink) Sync() error {
	return nil
}
