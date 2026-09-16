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
	ignoredMu sync.RWMutex
	// records is the single source of truth for both "is this host
	// ignored" and version/tombstone state, guarded by one lock so local
	// reads always match what was broadcast. See applyLocked, ApplyRemoteRecord.
	records     = make(map[string]IgnoredRecord)
	lastVersion int64
)

// IgnoredRecord is the versioned, tombstone-aware state of a single host,
// exchanged with peers via gossip broadcast and anti-entropy sync. Removed
// is a tombstone flag so a peer with a stale view can't resurrect a host
// once a newer removal is known.
type IgnoredRecord struct {
	Device  IgnoredDevice `json:"device"`
	Removed bool          `json:"removed"`
	Version int64         `json:"version"`
}

// Broadcaster is implemented by the clustering layer (e.g. gossip.Manager) to
// propagate ignored-host changes to the rest of the cluster. When nil, ignored
// host changes only apply to local in-memory state.
//
// version is already committed to local state by applyLocked - the
// clustering layer must only relay it, not allocate its own.
type Broadcaster interface {
	BroadcastAdd(device IgnoredDevice, version int64) error
	BroadcastRemove(host string, version int64) error
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

// BuildIgnoredDeviceEndpoint derives the canonical BMC API endpoint from a
// host name and model. This is the single source of truth for Endpoint -
// callers must NOT accept it from untrusted input (HTTP bodies, gossip
// messages), since TestConn combines it with real Vault credentials.
func BuildIgnoredDeviceEndpoint(name, model string) string {
	name = strings.TrimSpace(name)
	if model == moonshotModel {
		return "https://" + name + "/rest/v1/chassis/1"
	}
	return "https://" + name + "/redfish/v1/Chassis/"
}

// nextVersionLocked returns a strictly increasing logical clock value.
// Callers MUST hold ignoredMu for write.
func nextVersionLocked() int64 {
	v := time.Now().UnixNano()
	if v <= lastVersion {
		v = lastVersion + 1
	}
	lastVersion = v
	return v
}

// applyLocked allocates a version and commits this node's own add/remove as
// one atomic operation, under the same lock used for reads and for applying
// incoming gossip records (ApplyRemoteRecord). Returns the version so the
// caller can pass it, already-committed, to ClusterBroadcaster.
func applyLocked(hostname string, device IgnoredDevice, removed bool) int64 {
	ignoredMu.Lock()
	defer ignoredMu.Unlock()

	version := nextVersionLocked()
	records[hostname] = IgnoredRecord{Device: device, Removed: removed, Version: version}
	return version
}

// AddIgnoredDevice safely adds/updates a device in the local ignored-hosts map
// and, if clustering is enabled, broadcasts the change to the rest of the cluster.
//
// Endpoint is always derived from Name/Model, ignoring whatever was passed
// in device.Endpoint - see BuildIgnoredDeviceEndpoint.
func AddIgnoredDevice(device IgnoredDevice) {
	device.Name = strings.TrimSpace(device.Name)
	device.Endpoint = BuildIgnoredDeviceEndpoint(device.Name, device.Model)

	version := applyLocked(device.Name, device, false)

	if ClusterBroadcaster != nil {
		if err := ClusterBroadcaster.BroadcastAdd(device, version); err != nil {
			log = zap.L()
			log.Error("failed to broadcast ignored device add", zap.Error(err), zap.String("host", device.Name))
		}
	}
}

// RemoveIgnoredDeviceHost safely removes a host from the local ignored-hosts map
// and, if clustering is enabled, broadcasts the removal to the rest of the cluster.
func RemoveIgnoredDeviceHost(hostname string) {
	hostname = strings.TrimSpace(hostname)
	version := applyLocked(hostname, IgnoredDevice{Name: hostname}, true)

	if ClusterBroadcaster != nil {
		if err := ClusterBroadcaster.BroadcastRemove(hostname, version); err != nil {
			log = zap.L()
			log.Error("failed to broadcast ignored device removal", zap.Error(err), zap.String("host", hostname))
		}
	}
}

// ApplyRemoteRecord atomically applies an incoming add/remove from a peer
// (via NotifyMsg or MergeRemoteState), using last-write-wins conflict
// resolution keyed on version. Returns true if applied, false if stale.
//
// Version comparison and the local-map mutation happen under one lock
// (ignoredMu), the same lock applyLocked uses - memberlist may invoke
// delegate callbacks concurrently, so without a shared lock two operations
// for the same host could interleave and leave local state inconsistent
// with the version this node reported to its peers.
//
// device.Endpoint is always re-derived, ignoring whatever was received over
// the wire - see BuildIgnoredDeviceEndpoint.
func ApplyRemoteRecord(hostname string, device IgnoredDevice, removed bool, version int64) bool {
	device.Name = strings.TrimSpace(device.Name)
	device.Endpoint = BuildIgnoredDeviceEndpoint(device.Name, device.Model)

	ignoredMu.Lock()
	defer ignoredMu.Unlock()

	existing, ok := records[hostname]
	if ok && version <= existing.Version {
		return false
	}

	records[hostname] = IgnoredRecord{Device: device, Removed: removed, Version: version}
	if version > lastVersion {
		lastVersion = version
	}
	return true
}

// SnapshotIgnoredRecords returns a deep copy of the full versioned/tombstoned
// state, for anti-entropy full-state sync (memberlist's LocalState).
func SnapshotIgnoredRecords() map[string]IgnoredRecord {
	ignoredMu.RLock()
	defer ignoredMu.RUnlock()

	snapshot := make(map[string]IgnoredRecord, len(records))
	for k, v := range records {
		snapshot[k] = v
	}
	return snapshot
}

// IsIgnored returns true if the given host is currently on the ignored list
func IsIgnored(hostname string) bool {
	ignoredMu.RLock()
	defer ignoredMu.RUnlock()
	rec, ok := records[hostname]
	return ok && !rec.Removed
}

// GetIgnoredDevice returns the IgnoredDevice for a host, if present
func GetIgnoredDevice(hostname string) (IgnoredDevice, bool) {
	ignoredMu.RLock()
	defer ignoredMu.RUnlock()
	rec, ok := records[hostname]
	if !ok || rec.Removed {
		return IgnoredDevice{}, false
	}
	return rec.Device, true
}

// GetAllIgnoredDevices returns a snapshot slice of all currently ignored devices
func GetAllIgnoredDevices() []IgnoredDevice {
	ignoredMu.RLock()
	defer ignoredMu.RUnlock()
	devices := make([]IgnoredDevice, 0, len(records))
	for _, rec := range records {
		if rec.Removed {
			continue
		}
		devices = append(devices, rec.Device)
	}
	return devices
}

// IgnoredDeviceCount returns the number of currently ignored devices
func IgnoredDeviceCount() int {
	ignoredMu.RLock()
	defer ignoredMu.RUnlock()
	count := 0
	for _, rec := range records {
		if !rec.Removed {
			count++
		}
	}
	return count
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
