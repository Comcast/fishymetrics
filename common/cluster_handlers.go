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
	"encoding/json"
	"net/http"
	"strings"

	"go.uber.org/zap"
)

// ClusterMember describes a single node in the gossip cluster
type ClusterMember struct {
	Name string `json:"name"`
	Addr string `json:"addr"`
}

// ClusterInfo is implemented by the clustering layer (e.g. gossip.Manager) to
// report membership status. Set by main() when clustering is enabled.
type ClusterInfo interface {
	MemberCount() int
	ClusterMembers() []ClusterMember
	LocalNodeName() string
}

// ClusterStatusProvider is set by main() when clustering is enabled.
var ClusterStatusProvider ClusterInfo

// GossipAwareRemoveHost handles the /ignored/remove endpoint. Any node in the
// cluster (or the sole node when clustering is disabled) may service this
// request; the change is broadcast to the rest of the cluster via
// ClusterBroadcaster when clustering is enabled.
func GossipAwareRemoveHost(w http.ResponseWriter, r *http.Request) {
	var h host

	log = zap.L()

	body, err := getBody(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = unmarshalBody(body, &h, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	RemoveIgnoredDeviceHost(h.H)

	log.Info("removed host from ignored list", zap.String("host", h.H))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "removed", "host": h.H})
}

// GossipAwareAddHost handles adding a host to ignored devices. Any node in the
// cluster may service this request; the change is broadcast to the rest of
// the cluster via ClusterBroadcaster when clustering is enabled.
//
// The client-supplied Endpoint is ignored - AddIgnoredDevice always
// re-derives it from Name/Model, since trusting it would let a caller
// redirect a legitimate host's real Vault credentials (via TestConn) to a
// URL of their choosing.
func GossipAwareAddHost(w http.ResponseWriter, r *http.Request) {
	log = zap.L()

	var device IgnoredDevice
	if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	device.Name = strings.TrimSpace(device.Name)
	if device.Name == "" {
		http.Error(w, "\"Name\" is required", http.StatusBadRequest)
		return
	}

	AddIgnoredDevice(device)

	log.Info("added host to ignored list", zap.String("host", device.Name))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "added", "host": device.Name})
}

// ClusterStatus returns the current cluster membership status
func ClusterStatus(w http.ResponseWriter, r *http.Request) {
	if ClusterStatusProvider == nil {
		http.Error(w, "cluster not enabled", http.StatusServiceUnavailable)
		return
	}

	status := map[string]interface{}{
		"local_node":            ClusterStatusProvider.LocalNodeName(),
		"member_count":          ClusterStatusProvider.MemberCount(),
		"members":               ClusterStatusProvider.ClusterMembers(),
		"ignored_devices_count": IgnoredDeviceCount(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// ClusterHealth returns health status for the local cluster node
func ClusterHealth(w http.ResponseWriter, r *http.Request) {
	if ClusterStatusProvider == nil {
		http.Error(w, "cluster not enabled", http.StatusServiceUnavailable)
		return
	}

	health := map[string]interface{}{
		"status":       "healthy",
		"local_node":   ClusterStatusProvider.LocalNodeName(),
		"member_count": ClusterStatusProvider.MemberCount(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}
