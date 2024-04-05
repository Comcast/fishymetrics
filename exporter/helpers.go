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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/comcast/fishymetrics/common"
	"github.com/comcast/fishymetrics/oem"
	"github.com/hashicorp/go-retryablehttp"
)

func getChassisEndpoint(url, host string, client *retryablehttp.Client) (string, error) {
	var chas oem.Chassis
	var urlFinal string
	req := common.BuildRequest(url, host)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
		if resp.StatusCode == http.StatusUnauthorized {
			return "", common.ErrInvalidCredential
		} else {
			return "", fmt.Errorf("HTTP status %d", resp.StatusCode)
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Error reading Response Body - " + err.Error())
	}

	err = json.Unmarshal(body, &chas)
	if err != nil {
		return "", fmt.Errorf("Error Unmarshalling Chassis struct - " + err.Error())
	}

	if len(chas.Links.ManagerForServers.ServerManagerURLSlice) > 0 {
		urlFinal = chas.Links.ManagerForServers.ServerManagerURLSlice[0]
	}

	return urlFinal, nil
}

func getSystemsMetadata(url, host string, client *retryablehttp.Client) (oem.ServerManager, error) {
	var sm oem.ServerManager
	req := common.BuildRequest(url, host)

	resp, err := client.Do(req)
	if err != nil {
		return sm, err
	}
	defer resp.Body.Close()
	if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
		return sm, fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return sm, fmt.Errorf("Error reading Response Body - " + err.Error())
	}

	err = json.Unmarshal(body, &sm)
	if err != nil {
		return sm, fmt.Errorf("Error Unmarshalling ServerManager struct - " + err.Error())
	}

	return sm, nil
}

func getDIMMEndpoints(url, host string, client *retryablehttp.Client) (oem.Collection, error) {
	var dimms oem.Collection
	var resp *http.Response
	var err error
	retryCount := 0
	req := common.BuildRequest(url, host)

	resp, err = common.DoRequest(client, req)
	if err != nil {
		return dimms, err
	}
	defer resp.Body.Close()
	if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
		if resp.StatusCode == http.StatusNotFound {
			for retryCount < 1 && resp.StatusCode == http.StatusNotFound {
				time.Sleep(client.RetryWaitMin)
				resp, err = common.DoRequest(client, req)
				retryCount = retryCount + 1
			}
			if err != nil {
				return dimms, err
			} else if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
				return dimms, fmt.Errorf("HTTP status %d", resp.StatusCode)
			}
		} else {
			return dimms, fmt.Errorf("HTTP status %d", resp.StatusCode)
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return dimms, fmt.Errorf("Error reading Response Body - " + err.Error())
	}

	err = json.Unmarshal(body, &dimms)
	if err != nil {
		return dimms, fmt.Errorf("Error Unmarshalling Memory Collection struct - " + err.Error())
	}

	return dimms, nil
}

// The getDriveEndpoint function is used in a recursive fashion to get the body response
// of any type of drive, NVMe, Physical DiskDrives, or Logical Drives, using the GenericDrive struct
// This is used to find the final drive endpoints to append to the task pool for final scraping.
func getDriveEndpoint(url, host string, client *retryablehttp.Client) (oem.GenericDrive, error) {
	var drive oem.GenericDrive
	var resp *http.Response
	var err error
	retryCount := 0
	req := common.BuildRequest(url, host)
	resp, err = common.DoRequest(client, req)
	if err != nil {
		return drive, err
	}
	defer resp.Body.Close()
	if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
		if resp.StatusCode == http.StatusNotFound {
			for retryCount < 3 && resp.StatusCode == http.StatusNotFound {
				time.Sleep(client.RetryWaitMin)
				resp, err = common.DoRequest(client, req)
				retryCount = retryCount + 1
			}
			if err != nil {
				return drive, err
			} else if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
				return drive, fmt.Errorf("HTTP status %d", resp.StatusCode)
			}
		} else if resp.StatusCode == http.StatusUnauthorized {
			return drive, common.ErrInvalidCredential
		} else {
			return drive, fmt.Errorf("HTTP status %d", resp.StatusCode)
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return drive, fmt.Errorf("Error reading Response Body - " + err.Error())
	}

	err = json.Unmarshal(body, &drive)
	if err != nil {
		return drive, fmt.Errorf("Error Unmarshalling drive struct - " + err.Error())
	}

	return drive, nil
}

func getProcessorEndpoints(url, host string, client *retryablehttp.Client) (oem.Collection, error) {
	var processors oem.Collection
	var resp *http.Response
	var err error
	retryCount := 0
	req := common.BuildRequest(url, host)

	resp, err = common.DoRequest(client, req)
	if err != nil {
		return processors, err
	}
	defer resp.Body.Close()
	if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
		if resp.StatusCode == http.StatusNotFound {
			for retryCount < 3 && resp.StatusCode == http.StatusNotFound {
				time.Sleep(client.RetryWaitMin)
				resp, err = common.DoRequest(client, req)
				retryCount = retryCount + 1
			}
			if err != nil {
				return processors, err
			} else if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
				return processors, fmt.Errorf("HTTP status %d", resp.StatusCode)
			}
		} else if resp.StatusCode == http.StatusUnauthorized {
			return processors, common.ErrInvalidCredential
		} else {
			return processors, fmt.Errorf("HTTP status %d", resp.StatusCode)
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return processors, fmt.Errorf("Error reading Response Body - " + err.Error())
	}

	err = json.Unmarshal(body, &processors)
	if err != nil {
		return processors, fmt.Errorf("Error Unmarshalling Processors Collection struct - " + err.Error())
	}

	return processors, nil
}
