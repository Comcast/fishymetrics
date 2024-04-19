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
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"

	"github.com/comcast/fishymetrics/common"
	"github.com/comcast/fishymetrics/oem"
	"github.com/hashicorp/go-retryablehttp"
	"go.uber.org/zap"
)

func getMemberUrls(url, host string, client *retryablehttp.Client) ([]string, error) {
	var coll oem.Collection
	var urls []string
	req := common.BuildRequest(url, host)

	resp, err := client.Do(req)
	if err != nil {
		return urls, err
	}
	defer resp.Body.Close()
	if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
		if resp.StatusCode == http.StatusUnauthorized {
			return urls, common.ErrInvalidCredential
		} else {
			return urls, fmt.Errorf("HTTP status %d", resp.StatusCode)
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return urls, fmt.Errorf("Error reading Response Body - " + err.Error())
	}

	err = json.Unmarshal(body, &coll)
	if err != nil {
		return urls, fmt.Errorf("Error Unmarshalling Chassis struct - " + err.Error())
	}

	for _, member := range coll.Members {
		urls = append(urls, appendSlash(member.URL))
	}

	return urls, nil
}

func getSystemEndpoints(chassisUrls []string, host string, client *retryablehttp.Client, excludes Excludes) (SystemEndpoints, error) {
	var chas oem.Chassis
	var sysEnd SystemEndpoints

	for _, url := range chassisUrls {
		req := common.BuildRequest(url, host)

		resp, err := client.Do(req)
		if err != nil {
			return sysEnd, err
		}
		defer resp.Body.Close()
		if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
			if resp.StatusCode == http.StatusUnauthorized {
				return sysEnd, common.ErrInvalidCredential
			} else {
				return sysEnd, fmt.Errorf("HTTP status %d", resp.StatusCode)
			}
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return sysEnd, fmt.Errorf("Error reading Response Body - " + err.Error())
		}

		err = json.Unmarshal(body, &chas)
		if err != nil {
			return sysEnd, fmt.Errorf("Error Unmarshalling Chassis struct - " + err.Error())
		}

		// parse through Links to get the System Endpoints for each Chassis and only use unique URLs
		for _, sys := range chas.Links.System.LinksURLSlice {
			url := appendSlash(sys)
			if checkUnique(sysEnd.systems, url) {
				sysEnd.systems = append(sysEnd.systems, url)
			}
		}

		for _, sys := range chas.LinksLower.System.LinksURLSlice {
			url := appendSlash(sys)
			if checkUnique(sysEnd.systems, url) {
				sysEnd.systems = append(sysEnd.systems, url)
			}
		}

		for _, power := range chas.Links.Power.LinksURLSlice {
			url := appendSlash(power)
			if checkUnique(sysEnd.power, url) {
				sysEnd.power = append(sysEnd.power, url)
			}
		}

		for _, power := range chas.LinksLower.Power.LinksURLSlice {
			url := appendSlash(power)
			if checkUnique(sysEnd.power, url) {
				sysEnd.power = append(sysEnd.power, url)
			}
		}

		for _, thermal := range chas.Links.Thermal.LinksURLSlice {
			url := appendSlash(thermal)
			if checkUnique(sysEnd.thermal, url) {
				sysEnd.thermal = append(sysEnd.thermal, url)
			}
		}

		for _, thermal := range chas.LinksLower.Thermal.LinksURLSlice {
			url := appendSlash(thermal)
			if checkUnique(sysEnd.thermal, url) {
				sysEnd.thermal = append(sysEnd.thermal, url)
			}
		}

		for _, storage := range chas.Links.Storage.LinksURLSlice {
			url := appendSlash(storage)
			if checkUnique(sysEnd.storageController, url) {
				sysEnd.storageController = append(sysEnd.storageController, url)
			}
		}

		for _, storage := range chas.LinksLower.Storage.LinksURLSlice {
			url := appendSlash(storage)
			if checkUnique(sysEnd.storageController, url) {
				sysEnd.storageController = append(sysEnd.storageController, url)
			}
		}

		for _, drive := range chas.Links.Drives.LinksURLSlice {
			url := appendSlash(drive)
			// this list can potentially be large and cause scrapes to take a long time please
			// see the '--collector.drives.module-exclude' config in the README for more information
			if reg, ok := excludes["drive"]; ok {
				if !reg.(*regexp.Regexp).MatchString(url) {
					if checkUnique(sysEnd.drives, url) {
						sysEnd.drives = append(sysEnd.drives, url)
					}
				}
			} else {
				if checkUnique(sysEnd.drives, url) {
					sysEnd.drives = append(sysEnd.drives, url)
				}
			}
		}

		for _, drive := range chas.LinksLower.Drives.LinksURLSlice {
			url := appendSlash(drive)
			// this list can potentially be large and cause scrapes to take a long time please
			// see the '--collector.drives.module-exclude' config in the README for more information
			if reg, ok := excludes["drive"]; ok {
				if !reg.(*regexp.Regexp).MatchString(url) {
					if checkUnique(sysEnd.drives, url) {
						sysEnd.drives = append(sysEnd.drives, url)
					}
				}
			} else {
				if checkUnique(sysEnd.drives, url) {
					sysEnd.drives = append(sysEnd.drives, url)
				}
			}
		}
	}

	// check last resort places for power and thermal endpoints if none were found
	if len(sysEnd.power) == 0 && chas.PowerAlt.URL != "" {
		sysEnd.power = append(sysEnd.power, appendSlash(chas.PowerAlt.URL))
	}

	if len(sysEnd.thermal) == 0 && chas.ThermalAlt.URL != "" {
		sysEnd.thermal = append(sysEnd.thermal, appendSlash(chas.ThermalAlt.URL))
	}

	return sysEnd, nil
}

func getSystemsMetadata(url, host string, client *retryablehttp.Client) (oem.System, error) {
	var sys oem.System
	req := common.BuildRequest(url, host)

	resp, err := client.Do(req)
	if err != nil {
		return sys, err
	}
	defer resp.Body.Close()
	if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
		return sys, fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return sys, fmt.Errorf("Error reading Response Body - " + err.Error())
	}

	err = json.Unmarshal(body, &sys)
	if err != nil {
		return sys, fmt.Errorf("Error Unmarshalling ServerManager struct - " + err.Error())
	}

	return sys, nil
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

func getAllDriveEndpoints(ctx context.Context, fqdn, initialUrl, host string, client *retryablehttp.Client) (DriveEndpoints, error) {
	var driveEndpoints DriveEndpoints

	// Get initial JSON return of /redfish/v1/Systems/XXXX/SmartStorage/ArrayControllers/ set to output
	driveResp, err := getDriveEndpoint(initialUrl, host, client)
	if err != nil {
		log.Error("api call "+initialUrl+" failed - ", zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
		return driveEndpoints, err
	}

	// Loop through Members to get ArrayController URLs
	for _, member := range driveResp.Members {
		// for each ArrayController URL, get the JSON object
		// /redfish/v1/Systems/XXXX/SmartStorage/ArrayControllers/X/
		arrayCtrlResp, err := getDriveEndpoint(fqdn+member.URL, host, client)
		if err != nil {
			log.Error("api call "+fqdn+member.URL+" failed", zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
			return driveEndpoints, err
		}

		// If LogicalDrives is present, parse logical drive endpoint until all urls are found
		if arrayCtrlResp.LinksUpper.LogicalDrives.URL != "" {
			logicalDriveOutput, err := getDriveEndpoint(fqdn+arrayCtrlResp.LinksUpper.LogicalDrives.URL, host, client)
			if err != nil {
				log.Error("api call "+fqdn+arrayCtrlResp.LinksUpper.LogicalDrives.URL+" failed", zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
				return driveEndpoints, err
			}

			// loop through each Member in the "LogicalDrive" field
			for _, member := range logicalDriveOutput.Members {
				// append each URL in the Members array to the logicalDriveURLs array.
				driveEndpoints.logicalDriveURLs = append(driveEndpoints.logicalDriveURLs, appendSlash(member.URL))
			}
		}

		// If PhysicalDrives is present, parse physical drive endpoint until all urls are found
		if arrayCtrlResp.LinksUpper.PhysicalDrives.URL != "" {
			physicalDriveOutput, err := getDriveEndpoint(fqdn+arrayCtrlResp.LinksUpper.PhysicalDrives.URL, host, client)
			if err != nil {
				log.Error("api call "+fqdn+arrayCtrlResp.LinksUpper.PhysicalDrives.URL+" failed", zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
				return driveEndpoints, err
			}

			for _, member := range physicalDriveOutput.Members {
				driveEndpoints.physicalDriveURLs = append(driveEndpoints.physicalDriveURLs, appendSlash(member.URL))
			}
		}

		// If LogicalDrives is present, parse logical drive endpoint until all urls are found
		if arrayCtrlResp.LinksLower.LogicalDrives.URL != "" {
			logicalDriveOutput, err := getDriveEndpoint(fqdn+arrayCtrlResp.LinksLower.LogicalDrives.URL, host, client)
			if err != nil {
				log.Error("api call "+fqdn+arrayCtrlResp.LinksLower.LogicalDrives.URL+" failed", zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
				return driveEndpoints, err
			}

			// loop through each Member in the "LogicalDrive" field
			for _, member := range logicalDriveOutput.Members {
				// append each URL in the Members array to the logicalDriveURLs array.
				driveEndpoints.logicalDriveURLs = append(driveEndpoints.logicalDriveURLs, appendSlash(member.URL))
			}
		}

		// If PhysicalDrives is present, parse physical drive endpoint until all urls are found
		if arrayCtrlResp.LinksLower.PhysicalDrives.URL != "" {
			physicalDriveOutput, err := getDriveEndpoint(fqdn+arrayCtrlResp.LinksLower.PhysicalDrives.URL, host, client)
			if err != nil {
				log.Error("api call "+fqdn+arrayCtrlResp.LinksLower.PhysicalDrives.URL+" failed", zap.Error(err), zap.Any("trace_id", ctx.Value("traceID")))
				return driveEndpoints, err
			}

			for _, member := range physicalDriveOutput.Members {
				driveEndpoints.physicalDriveURLs = append(driveEndpoints.physicalDriveURLs, appendSlash(member.URL))
			}
		}
	}

	return driveEndpoints, nil
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

// appendSlash appends a slash to the end of a URL if it does not already have one
func appendSlash(url string) string {
	if url[len(url)-1] != '/' {
		return url + "/"
	}
	return url
}

func checkUnique(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return false
		}
	}
	return true
}
