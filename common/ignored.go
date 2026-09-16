/*
 * Copyright 2026 Comcast Cable Communications Management, LLC
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
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/comcast/fishymetrics/config"
	"go.uber.org/zap"
)

var (
	ignoredMu      sync.RWMutex
	IgnoredDevices = make(map[string]IgnoredDevice)
)

// Broadcaster is implemented by the clustering layer (e.g. gossip.Manager) to
// propagate ignored-host changes to the rest of the cluster. When nil, ignored
// host changes only apply to local in-memory state.
type Broadcaster interface {
	BroadcastAdd(device IgnoredDevice) error
	BroadcastRemove(host string) error
}

// ClusterBroadcaster is set by main() when clustering is enabled. All calls to
// AddIgnoredDevice/RemoveIgnoredDeviceHost will broadcast to the cluster when set.
var ClusterBroadcaster Broadcaster

type host struct {
	H string `json:"host"`
}

type IgnoredDevice struct {
	Name              string
	Endpoint          string
	Model             string
	CredentialProfile string
}

// moonshotModel matches exporter/moonshot.MOONSHOT. Duplicated here (rather
// than imported) to avoid a circular dependency, since exporter/moonshot
// imports common.
const moonshotModel = "Moonshot"

// BuildIgnoredDeviceEndpoint derives the canonical BMC API endpoint for a
// given host name and hardware model. This is the single source of truth
// for how an IgnoredDevice's Endpoint is computed, and callers MUST NOT
// accept an Endpoint value from untrusted input (HTTP request bodies,
// gossip messages from peers, etc.) — TestConn combines the stored Endpoint
// with real Vault-backed BMC credentials resolved for Name, so an
// independently-controllable Endpoint would allow an attacker to redirect
// those credentials to a host of their choosing. See AddIgnoredDevice and
// SetIgnoredDeviceLocal, which both enforce this derivation.
func BuildIgnoredDeviceEndpoint(name, model string) string {
	name = strings.TrimSpace(name)
	if model == moonshotModel {
		return "https://" + name + "/rest/v1/chassis/1"
	}
	return "https://" + name + "/redfish/v1/Chassis/"
}

// AddIgnoredDevice safely adds/updates a device in the local ignored-hosts map
// and, if clustering is enabled, broadcasts the change to the rest of the cluster.
//
// Endpoint is always derived from Name/Model via BuildIgnoredDeviceEndpoint
// regardless of what was passed in device.Endpoint - see that function for
// why this is a security boundary, not just a convenience.
func AddIgnoredDevice(device IgnoredDevice) {
	device.Name = strings.TrimSpace(device.Name)
	device.Endpoint = BuildIgnoredDeviceEndpoint(device.Name, device.Model)

	ignoredMu.Lock()
	IgnoredDevices[device.Name] = device
	ignoredMu.Unlock()

	if ClusterBroadcaster != nil {
		if err := ClusterBroadcaster.BroadcastAdd(device); err != nil {
			log = zap.L()
			log.Error("failed to broadcast ignored device add", zap.Error(err), zap.String("host", device.Name))
		}
	}
}

// RemoveIgnoredDeviceHost safely removes a host from the local ignored-hosts map
// and, if clustering is enabled, broadcasts the removal to the rest of the cluster.
func RemoveIgnoredDeviceHost(hostname string) {
	ignoredMu.Lock()
	delete(IgnoredDevices, hostname)
	ignoredMu.Unlock()

	if ClusterBroadcaster != nil {
		if err := ClusterBroadcaster.BroadcastRemove(hostname); err != nil {
			log = zap.L()
			log.Error("failed to broadcast ignored device removal", zap.Error(err), zap.String("host", hostname))
		}
	}
}

// SetIgnoredDeviceLocal updates the local ignored-hosts map only, without
// broadcasting to the cluster. Used by the clustering layer itself (e.g.
// gossip anti-entropy merge / incoming gossip messages) where the change is
// already being disseminated through the cluster and re-broadcasting would
// cause redundant traffic or feedback loops.
//
// Endpoint is always re-derived from Name/Model via BuildIgnoredDeviceEndpoint,
// regardless of what was received over the wire from a peer - a compromised
// or misbehaving peer must not be able to inject an arbitrary Endpoint that
// gets combined with real Vault credentials by TestConn.
func SetIgnoredDeviceLocal(device IgnoredDevice) {
	device.Name = strings.TrimSpace(device.Name)
	device.Endpoint = BuildIgnoredDeviceEndpoint(device.Name, device.Model)

	ignoredMu.Lock()
	IgnoredDevices[device.Name] = device
	ignoredMu.Unlock()
}

// UnsetIgnoredDeviceLocal removes a host from the local ignored-hosts map
// only, without broadcasting to the cluster. See SetIgnoredDeviceLocal.
func UnsetIgnoredDeviceLocal(hostname string) {
	ignoredMu.Lock()
	delete(IgnoredDevices, hostname)
	ignoredMu.Unlock()
}

// IsIgnored returns true if the given host is currently on the ignored list
func IsIgnored(hostname string) bool {
	ignoredMu.RLock()
	defer ignoredMu.RUnlock()
	_, ok := IgnoredDevices[hostname]
	return ok
}

// GetIgnoredDevice returns the IgnoredDevice for a host, if present
func GetIgnoredDevice(hostname string) (IgnoredDevice, bool) {
	ignoredMu.RLock()
	defer ignoredMu.RUnlock()
	d, ok := IgnoredDevices[hostname]
	return d, ok
}

// GetAllIgnoredDevices returns a snapshot slice of all currently ignored devices
func GetAllIgnoredDevices() []IgnoredDevice {
	ignoredMu.RLock()
	defer ignoredMu.RUnlock()
	devices := make([]IgnoredDevice, 0, len(IgnoredDevices))
	for _, d := range IgnoredDevices {
		devices = append(devices, d)
	}
	return devices
}

// IgnoredDeviceCount returns the number of currently ignored devices
func IgnoredDeviceCount() int {
	ignoredMu.RLock()
	defer ignoredMu.RUnlock()
	return len(IgnoredDevices)
}

func TestConn(w http.ResponseWriter, r *http.Request) {
	var h host
	var path string
	response := make(map[string]interface{})
	response["connectionTest"] = false

	log = zap.L()

	body, err := getBody(r)
	if err != nil {
		response["error"] = err.Error()
		resp, _ := marshalResponse(&response, r)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(resp)
		return
	}

	err = unmarshalBody(body, &h, r)
	if err != nil {
		log.Error("failed to unmarshal body from frontend", zap.Error(err), zap.String("path", r.URL.Path))
		response["error"] = err.Error()
		resp, _ := marshalResponse(&response, r)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(resp)
		return
	}

	device, ok := GetIgnoredDevice(h.H)
	if !ok {
		log.Error("missing host from ignored hosts list", zap.Error(err), zap.String("path", r.URL.Path))
		response["error"] = "missing host from ignored hosts list"
		resp, _ := marshalResponse(&response, r)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(resp)
		return
	}
	path = BuildIgnoredDeviceEndpoint(device.Name, device.Model)
	credProfile := device.CredentialProfile
	// get credentials from vault
	credential, err := ChassisCreds.GetCredentials(context.Background(), credProfile, h.H)
	if err != nil {
		log.Error("issue retrieving credentials from vault using target "+h.H, zap.Error(err), zap.String("path", r.URL.Path))
		response["error"] = err.Error()
		resp, _ := marshalResponse(&response, r)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(resp)
		return
	}

	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		log.Error("failed to build test connection request", zap.Error(err), zap.String("path", r.URL.Path))
		response["error"] = err.Error()
		resp, _ := marshalResponse(&response, r)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(resp)
		return
	}
	req.SetBasicAuth(credential.User, credential.Pass)

	tr := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 3 * time.Second,
		}).Dial,
		MaxIdleConns:          1,
		MaxConnsPerHost:       1,
		MaxIdleConnsPerHost:   1,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.GetConfig().SSLVerify,
		},
		TLSHandshakeTimeout: 10 * time.Second,
	}

	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: tr,
	}

	res, err := client.Do(req)
	if err != nil {
		log.Error("request failed for test connection call", zap.Error(err), zap.String("path", r.URL.Path))
		response["error"] = err.Error()
		resp, _ := marshalResponse(&response, r)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(resp)
		return
	}

	// Ensure response body is closed
	if res != nil && res.Body != nil {
		defer res.Body.Close()
	}

	// Check if response is nil
	if res == nil {
		log.Error("received nil response for test connection call", zap.String("path", r.URL.Path))
		response["error"] = "received nil response from target"
		resp, _ := marshalResponse(&response, r)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(resp)
		return
	}

	if res.StatusCode != 401 {
		response["connectionTest"] = true
	} else {
		response["error"] = res.Status
	}

	resp, _ := marshalResponse(&response, r)
	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

func RemoveHost(w http.ResponseWriter, r *http.Request) {
	var h host

	log = zap.L()

	body, err := getBody(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	err = unmarshalBody(body, &h, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	RemoveIgnoredDeviceHost(h.H)
	log.Info("remove host " + h.H + " from ignored list")
	w.WriteHeader(http.StatusOK)
}

func getBody(r *http.Request) ([]byte, error) {
	var body []byte
	log = zap.L()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error("could not parse request body", zap.Error(err), zap.String("path", r.URL.Path))
		return body, err
	}
	return body, nil
}

func unmarshalBody(b []byte, h *host, r *http.Request) error {
	var err error
	log = zap.L()

	err = json.Unmarshal(b, h)
	if err != nil {
		log.Error("could not unmarshal host struct", zap.Error(err), zap.String("path", r.URL.Path))
		return err
	}
	return nil
}

func marshalResponse(p *map[string]interface{}, r *http.Request) ([]byte, error) {
	var resp []byte
	log = zap.L()

	resp, err := json.Marshal(p)
	if err != nil {
		log.Error("could not marshal response", zap.Error(err), zap.String("path", r.URL.Path))
		return resp, err
	}
	return resp, nil
}
