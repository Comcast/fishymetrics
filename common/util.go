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

package common

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/comcast/fishymetrics/config"

	"github.com/hashicorp/go-retryablehttp"
)

var (
	ErrInvalidCredential = errors.New("invalid credential")
)

type metricHandler func([]byte) error
type Handler metricHandler

func Fetch(uri, host, profile string, client *retryablehttp.Client) func() ([]byte, error) {
	var resp *http.Response
	var credential *Credential
	var err error
	retryCount := 0

	return func() ([]byte, error) {
		// Add a 100 milliseconds delay in between requests because cisco devices respond in a non idiomatic manner
		time.Sleep(100 * time.Millisecond)
		req := BuildRequest(uri, host)
		resp, err = DoRequest(client, req)
		if err != nil {
			return nil, err
		}
		defer EmptyAndCloseBody(resp)
		if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
			if resp.StatusCode == http.StatusNotFound {
				for retryCount < 3 && resp.StatusCode == http.StatusNotFound {
					time.Sleep(client.RetryWaitMin)
					resp, err = DoRequest(client, req)
					if err != nil {
						return nil, err
					}
					defer EmptyAndCloseBody(resp)
					retryCount = retryCount + 1
				}
				if err != nil {
					return nil, err
				} else if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
					return nil, fmt.Errorf("HTTP status %d", resp.StatusCode)
				}
			} else if resp.StatusCode == http.StatusUnauthorized {
				if ChassisCreds.Vault != nil {
					// Credentials may have rotated, go to vault and get the latest
					credential, err = ChassisCreds.GetCredentials(context.Background(), profile, host)
					if err != nil {
						return nil, fmt.Errorf("issue retrieving credentials from vault using target: %s", host)
					}
					ChassisCreds.Set(host, credential)
				} else {
					return nil, ErrInvalidCredential
				}

				// build new request with updated credentials
				req = BuildRequest(uri, host)

				time.Sleep(client.RetryWaitMin)
				resp, err = DoRequest(client, req)
				if err != nil {
					return nil, fmt.Errorf("Retry DoRequest failed - " + err.Error())
				}
				defer EmptyAndCloseBody(resp)
				if resp.StatusCode == http.StatusUnauthorized {
					return nil, ErrInvalidCredential
				}
			} else {
				return nil, fmt.Errorf("HTTP status %d", resp.StatusCode)
			}
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("Error reading Response Body - " + err.Error())
		}
		return body, err
	}
}

// This is required to have a proper cleanup of the response body
// to have correctly working keep-alive connections
func EmptyAndCloseBody(resp *http.Response) {
	if resp.Body != nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

func BuildRequest(uri, host string) *retryablehttp.Request {
	var user, password string

	if c, ok := ChassisCreds.Get(host); ok {
		credential := c
		user = credential.User
		password = credential.Pass
	} else {
		// use statically configured credentials
		user = config.GetConfig().User
		password = config.GetConfig().Pass
	}

	req, _ := retryablehttp.NewRequest(http.MethodGet, uri, nil)
	req.SetBasicAuth(user, password)
	// this header is required by iDRAC9 with FW ver. 3.xx and 4.xx
	req.Header.Add("Accept", "application/json")

	return req
}

func DoRequest(client *retryablehttp.Client, req *retryablehttp.Request) (*http.Response, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
