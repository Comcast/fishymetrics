/*
 * Copyright 2025 Comcast Cable Communications Management, LLC
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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/comcast/fishymetrics/common"
	"github.com/comcast/fishymetrics/config"
	"github.com/comcast/fishymetrics/middleware/logging"
	"github.com/comcast/fishymetrics/oem"
	"github.com/comcast/fishymetrics/pool"
	"github.com/hashicorp/go-retryablehttp"
	"go.uber.org/zap"
)

// ComponentType represents the types of components that can be scraped
type ComponentType string

const (
	ComponentThermal           ComponentType = "thermal"
	ComponentPower             ComponentType = "power"
	ComponentMemory            ComponentType = "memory"
	ComponentProcessor         ComponentType = "processor"
	ComponentDrives            ComponentType = "drives"
	ComponentStorageController ComponentType = "storage_controller"
	ComponentFirmware          ComponentType = "firmware"
	ComponentSystem            ComponentType = "system"
)

// ValidComponents contains all valid component types for partial scraping
var ValidComponents = map[ComponentType]bool{
	ComponentThermal:           true,
	ComponentPower:             true,
	ComponentMemory:            true,
	ComponentProcessor:         true,
	ComponentDrives:            true,
	ComponentStorageController: true,
	ComponentFirmware:          true,
	ComponentSystem:            true,
}

// ParseComponents parses a comma-separated list of components and validates them
// Invalid components are silently ignored
func ParseComponents(componentsStr string) ([]ComponentType, error) {
	if componentsStr == "" {
		return nil, errors.New("components parameter is empty")
	}

	parts := strings.Split(componentsStr, ",")
	components := make([]ComponentType, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(strings.ToLower(part))
		if trimmed == "" {
			continue // Skip empty strings
		}

		component := ComponentType(trimmed)
		if ValidComponents[component] {
			components = append(components, component)
		}
		// Silently ignore invalid components
	}

	if len(components) == 0 {
		return nil, fmt.Errorf("no valid components specified. Valid components are: thermal, power, memory, processor, drives, storage_controller, firmware, system")
	}

	return components, nil
}

// NewPartialExporter creates an exporter that only collects specified components
func NewPartialExporter(ctx context.Context, target, uri, profile, model string,
	excludes Excludes, components []ComponentType, plugins ...Plugin) (*Exporter, error) {

	var u *url.URL
	var tasks []*pool.Task
	var exp = Exporter{
		ctx:           ctx,
		credProfile:   profile,
		DeviceMetrics: NewDeviceMetrics(),
		Model:         model,
	}

	log = zap.L()

	// Create a map for quick component lookup
	componentMap := make(map[ComponentType]bool)
	for _, c := range components {
		componentMap[c] = true
	}

	retryClient := NewHTTPClient(ctx)
	retryClient.RequestLogHook = func(l retryablehttp.Logger, r *http.Request, i int) {
		retryCount := i
		if retryCount > 0 {
			log.Error("api call "+r.URL.String()+" failed, retry #"+strconv.Itoa(retryCount), zap.Any("trace_id", ctx.Value(logging.TraceIDKey("traceID"))))
		}
	}

	// Check that the target passed in has http:// or https:// prefixed
	u, err := url.ParseRequestURI(target)
	if err != nil || u.Host == "" {
		u, err = url.ParseRequestURI(config.GetConfig().BMCScheme + "://" + target)
		if err != nil {
			log.Error("error parsing target param", zap.Error(err), zap.String("target", target),
				zap.Any("trace_id", ctx.Value(logging.TraceIDKey("traceID"))))
			return nil, err
		}
	}

	exp.host = u.Hostname()
	exp.url = u.String()
	exp.client = retryClient

	// Get base endpoints
	chassisEndpoints, err := getMemberUrls(exp.url+uri+"/Chassis/", target, retryClient)
	if err != nil {
		// Check if device should be ignored
		if errors.Is(err, common.ErrInvalidCredential) {
			common.IgnoredDevices[exp.host] = common.IgnoredDevice{
				Name:              exp.host,
				Endpoint:          "https://" + exp.host + "/redfish/v1/Chassis/",
				Model:             exp.Model,
				CredentialProfile: exp.credProfile,
			}
			log.Info("added host "+exp.host+" to ignored list",
				zap.Any("trace_id", exp.ctx.Value(logging.TraceIDKey("traceID"))))
			var upMetric = (*exp.DeviceMetrics)["up"]
			(*upMetric)["up"].WithLabelValues().Set(float64(2))
			return &exp, nil
		}
		return nil, err
	}

	// Get manager endpoints if firmware is requested
	var mgrEndpointFinal string
	if componentMap[ComponentFirmware] || componentMap[ComponentSystem] {
		mgrEndpoints, err := getMemberUrls(exp.url+uri+"/Managers/", target, retryClient)
		if err != nil {
			log.Error("error when getting manager endpoint", zap.Error(err),
				zap.Any("trace_id", ctx.Value(logging.TraceIDKey("traceID"))))
			return nil, err
		}

		if len(mgrEndpoints) > 1 {
			for _, member := range mgrEndpoints {
				if !strings.Contains(member, "CIMC") {
					mgrEndpointFinal = member
					break
				}
			}
		} else if len(mgrEndpoints) > 0 {
			mgrEndpointFinal = mgrEndpoints[0]
		}
	}

	// Prepare chassis URLs
	var chasUrlsFinal []string
	for _, chasUrl := range chassisEndpoints {
		chasUrlsFinal = append(chasUrlsFinal, exp.url+chasUrl)
	}

	// Get system endpoints based on requested components
	needSystemEndpoints := componentMap[ComponentPower] || componentMap[ComponentThermal] ||
		componentMap[ComponentDrives] || componentMap[ComponentStorageController] ||
		componentMap[ComponentMemory] || componentMap[ComponentProcessor] || componentMap[ComponentSystem]

	var sysEndpoints SystemEndpoints
	var sysResp oem.System

	if needSystemEndpoints {
		sysEndpoints, err = getSystemEndpoints(chasUrlsFinal, target, retryClient, excludes)
		if err != nil {
			log.Error("error when getting chassis endpoints", zap.Error(err),
				zap.Any("trace_id", ctx.Value(logging.TraceIDKey("traceID"))))
			return nil, err
		}

		if len(sysEndpoints.systems) > 0 {
			sysResp, err = getSystemsMetadata(exp.url+sysEndpoints.systems[0], target, retryClient)
			if err != nil {
				log.Error("error when getting BIOS version", zap.Error(err),
					zap.Any("trace_id", ctx.Value(logging.TraceIDKey("traceID"))))
				return nil, err
			}
			exp.biosVersion = sysResp.BiosVersion
			exp.ChassisSerialNumber = strings.TrimRight(sysResp.SerialNumber, " ")
			exp.systemHostname = sysResp.SystemHostname
		}
	}

	// Add tasks based on requested components

	// System info (always included if system component is requested)
	if componentMap[ComponentSystem] && len(sysEndpoints.systems) > 0 {
		tasks = append(tasks,
			pool.NewTask(common.Fetch(exp.url+sysEndpoints.systems[0], target, profile, retryClient),
				exp.url+sysEndpoints.systems[0],
				handle(&exp, MEMORY_SUMMARY, STORAGEBATTERY)))
	}

	// Power metrics
	if componentMap[ComponentPower] {
		for _, url := range sysEndpoints.power {
			tasks = append(tasks,
				pool.NewTask(common.Fetch(exp.url+url, target, profile, retryClient),
					exp.url+url, handle(&exp, POWER)))
		}
	}

	// Thermal metrics
	if componentMap[ComponentThermal] {
		for _, url := range sysEndpoints.thermal {
			tasks = append(tasks,
				pool.NewTask(common.Fetch(exp.url+url, target, profile, retryClient),
					exp.url+url, handle(&exp, THERMAL)))
		}
	}

	// Memory metrics
	if componentMap[ComponentMemory] && len(sysEndpoints.systems) > 0 {
		var systemMemoryEndpoint = GetMemoryURL(sysResp)
		if systemMemoryEndpoint != "" {
			dimms, err := getDIMMEndpoints(exp.url+systemMemoryEndpoint, target, retryClient)
			if err != nil {
				log.Error("error when getting DIMM endpoints", zap.Error(err),
					zap.Any("trace_id", ctx.Value(logging.TraceIDKey("traceID"))))
			} else {
				for _, dimm := range dimms.Members {
					tasks = append(tasks,
						pool.NewTask(common.Fetch(exp.url+dimm.URL, target, profile, retryClient),
							exp.url+dimm.URL, handle(&exp, MEMORY)))
				}
			}
		}
	}

	// Processor metrics
	if componentMap[ComponentProcessor] && len(sysEndpoints.systems) > 0 {
		processors, err := getProcessorEndpoints(exp.url+sysEndpoints.systems[0]+"Processors/", target, retryClient)
		if err != nil {
			log.Error("error when getting Processors endpoints", zap.Error(err),
				zap.Any("trace_id", ctx.Value(logging.TraceIDKey("traceID"))))
		} else {
			for _, processor := range processors.Members {
				tasks = append(tasks,
					pool.NewTask(common.Fetch(exp.url+processor.URL, target, profile, retryClient),
						exp.url+processor.URL, handle(&exp, PROCESSOR)))
			}
		}
	}

	// Storage controller & drive metrics
	if componentMap[ComponentStorageController] || componentMap[ComponentDrives] {
		// From SmartStorage
		var ss = GetSmartStorageURL(sysResp)
		var driveEndpointsResp DriveEndpoints

		if ss != "" {
			driveEndpointsResp, err = getAllDriveEndpoints(ctx, exp.url, exp.url+ss, target, retryClient, excludes)
			if err != nil {
				log.Error("error when getting drive endpoints", zap.Error(err), zap.Any("trace_id", ctx.Value(logging.TraceIDKey("traceID"))))
				return nil, err
			}
		}

		if (len(sysEndpoints.storageController) == 0 && ss == "") || (len(sysEndpoints.drives) == 0 && len(driveEndpointsResp.physicalDriveURLs) == 0) {
			if sysResp.Storage.URL != "" {
				url := appendSlash(sysResp.Storage.URL)
				driveEndpointsResp, err = getAllDriveEndpoints(ctx, exp.url, exp.url+url, target, retryClient, excludes)
				if err != nil {
					log.Error("error when getting drive endpoints", zap.Error(err), zap.Any("trace_id", ctx.Value(logging.TraceIDKey("traceID"))))
					return nil, err
				}
			}
		}

		if componentMap[ComponentStorageController] {
			log.Debug("partial storage controller endpoints", zap.Strings("array_controller_endpoints", driveEndpointsResp.arrayControllerURLs),
				zap.Strings("storage_ctrl_endpoints", sysEndpoints.storageController), zap.Any("trace_id", ctx.Value(logging.TraceIDKey("traceID"))))

			// System Endpoints
			for _, url := range sysEndpoints.storageController {
				tasks = append(tasks, pool.NewTask(common.Fetch(exp.url+url, target, profile, retryClient),
					exp.url+url, handle(&exp, STORAGE_CONTROLLER)))
			}
			// SmartStorage or Storage endpoints
			for _, url := range driveEndpointsResp.arrayControllerURLs {
				tasks = append(tasks, pool.NewTask(common.Fetch(exp.url+url, target, profile, retryClient), exp.url+url, handle(&exp, STORAGE_CONTROLLER)))
			}
		}

		if componentMap[ComponentDrives] {
			log.Debug("partial drive endpoints", zap.Strings("logical_drive_endpoints", driveEndpointsResp.logicalDriveURLs),
				zap.Strings("virtual_drives_endpoints", sysEndpoints.virtualDrives),
				zap.Strings("drives_endpoints", sysEndpoints.drives),
				zap.Strings("physical_drive_endpoints", driveEndpointsResp.physicalDriveURLs),
				zap.Any("trace_id", ctx.Value(logging.TraceIDKey("traceID"))))

			// virtual drives
			for _, url := range sysEndpoints.virtualDrives {
				tasks = append(tasks, pool.NewTask(common.Fetch(exp.url+url, target, profile, retryClient),
					exp.url+url, handle(&exp, LOGICALDRIVE)))
			}
			// Logical drives
			for _, url := range driveEndpointsResp.logicalDriveURLs {
				tasks = append(tasks, pool.NewTask(common.Fetch(exp.url+url, target, profile, retryClient),
					exp.url+url, handle(&exp, LOGICALDRIVE)))
			}

			// System Endpoint
			for _, url := range sysEndpoints.drives {
				tasks = append(tasks, pool.NewTask(common.Fetch(exp.url+url, target, profile, retryClient),
					exp.url+url, handle(&exp, UNKNOWN_DRIVE)))
			}
			// SmartStorage or Storage endpoints
			for _, url := range driveEndpointsResp.physicalDriveURLs {
				tasks = append(tasks, pool.NewTask(common.Fetch(exp.url+url, target, profile, retryClient),
					exp.url+url, handle(&exp, UNKNOWN_DRIVE)))
			}
		}
	}

	// Firmware metrics
	if componentMap[ComponentFirmware] {
		// Manager firmware
		if mgrEndpointFinal != "" {
			tasks = append(tasks,
				pool.NewTask(common.Fetch(exp.url+mgrEndpointFinal, target, profile, retryClient),
					exp.url+mgrEndpointFinal,
					handle(&exp, FIRMWARE, ILOSELFTEST)))
		}

		// Try to get firmware inventory from Systems endpoint first
		var systemFML = GetFirmwareInventoryURL(sysResp)
		var firmwareInventoryEndpoints []string
		if systemFML != "" {
			tasks = append(tasks, pool.NewTask(common.Fetch(exp.url+systemFML, target, profile, retryClient),
				exp.url+systemFML, handle(&exp, FIRMWAREINVENTORY)))
		} else {
			// Check for /redfish/v1/Managers/XXXX/UpdateService/ for firmware inventory URL
			rootComponents, err := getSystemsMetadata(exp.url+uri, target, retryClient)
			if err == nil && rootComponents.UpdateService.URL != "" {
				updateServiceEndpoints, err := getSystemsMetadata(exp.url+rootComponents.UpdateService.URL, target, retryClient)
				if err == nil {
					if len(updateServiceEndpoints.FirmwareInventory.LinksURLSlice) == 1 {
						firmwareInventoryEndpoints, err = getMemberUrls(exp.url+updateServiceEndpoints.FirmwareInventory.LinksURLSlice[0], target, retryClient)
						if err != nil {
							log.Error("error when getting firmware inventory endpoints", zap.Error(err), zap.Any("trace_id", ctx.Value(logging.TraceIDKey("traceID"))))
							return nil, err
						}
					} else if len(updateServiceEndpoints.FirmwareInventory.LinksURLSlice) > 1 {
						firmwareInventoryEndpoints = updateServiceEndpoints.FirmwareInventory.LinksURLSlice
					}

					if len(firmwareInventoryEndpoints) < 75 {
						for _, fwEp := range firmwareInventoryEndpoints {
							if reg, ok := excludes["firmware"]; ok {
								if !reg.(*regexp.Regexp).MatchString(fwEp) {
									tasks = append(tasks,
										pool.NewTask(common.Fetch(exp.url+fwEp, target, profile, retryClient),
											exp.url+fwEp, handle(&exp, FIRMWAREINVENTORY)))
								}
							} else {
								tasks = append(tasks,
									pool.NewTask(common.Fetch(exp.url+fwEp, target, profile, retryClient),
										exp.url+fwEp, handle(&exp, FIRMWAREINVENTORY)))
							}
						}
					}
				}
			}
		}
	}

	exp.pool = pool.NewPool(tasks, 1)

	// Apply plugins if provided
	for _, plugin := range plugins {
		err := plugin.Apply(&exp)
		if err != nil {
			return &exp, err
		}
	}

	return &exp, nil
}
