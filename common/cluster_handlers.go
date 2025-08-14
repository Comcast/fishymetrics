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

package common

import (
	"encoding/json"
	"net/http"

	"github.com/comcast/fishymetrics/raft"
	"go.uber.org/zap"
)

var ClusterManager *raft.ClusterManager

// ClusterAwareRemoveHost handles the /ignored/remove endpoint with cluster awareness
func ClusterAwareRemoveHost(w http.ResponseWriter, r *http.Request) {
	var h host

	log = zap.L()

	// Check if we have a cluster manager
	if ClusterManager == nil {
		// Fall back to non-clustered behavior
		RemoveHost(w, r)
		return
	}

	// Check if this node is the leader
	if !ClusterManager.IsLeader() {
		leader := ClusterManager.GetLeader()
		response := map[string]interface{}{
			"error":  "This node is not the leader. Write operations must be performed on the leader.",
			"leader": leader,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTemporaryRedirect)
		json.NewEncoder(w).Encode(response)
		return
	}

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

	// Apply the change through Raft consensus
	if err := ClusterManager.RemoveIgnoredDevice(h.H); err != nil {
		log.Error("failed to remove device through consensus", zap.Error(err), zap.String("device", h.H))
		http.Error(w, "Failed to remove device: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Info("removed host from ignored list via consensus", zap.String("host", h.H))
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "removed", "host": h.H})
}

// ClusterAwareAddHost handles adding a host to ignored devices with cluster awareness
func ClusterAwareAddHost(w http.ResponseWriter, r *http.Request) {
	log = zap.L()

	// Check if we have a cluster manager
	if ClusterManager == nil {
		http.Error(w, "Cluster manager not initialized", http.StatusInternalServerError)
		return
	}

	// Check if this node is the leader
	if !ClusterManager.IsLeader() {
		leader := ClusterManager.GetLeader()
		response := map[string]interface{}{
			"error":  "This node is not the leader. Write operations must be performed on the leader.",
			"leader": leader,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTemporaryRedirect)
		json.NewEncoder(w).Encode(response)
		return
	}

	var device IgnoredDevice
	if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Apply the change through Raft consensus
	if err := ClusterManager.AddIgnoredDevice(device); err != nil {
		log.Error("failed to add device through consensus", zap.Error(err), zap.String("device", device.Name))
		http.Error(w, "Failed to add device: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Info("added host to ignored list via consensus", zap.String("host", device.Name))
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "added", "host": device.Name})
}

// ClusterStatus returns the current cluster status
func ClusterStatus(w http.ResponseWriter, r *http.Request) {
	if ClusterManager == nil {
		http.Error(w, "Cluster manager not initialized", http.StatusServiceUnavailable)
		return
	}

	status := map[string]interface{}{
		"is_leader": ClusterManager.IsLeader(),
		"leader":    ClusterManager.GetLeader(),
	}

	// Add shared state information
	state := ClusterManager.GetState()
	if state != nil {
		status["shared_state"] = map[string]interface{}{
			"ignored_devices_count": len(state.IgnoredDevices),
			"has_vault_token":       state.VaultToken != nil,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// ClusterHealth returns health status for the cluster
func ClusterHealth(w http.ResponseWriter, r *http.Request) {
	if ClusterManager == nil {
		http.Error(w, "Cluster manager not initialized", http.StatusServiceUnavailable)
		return
	}

	health := map[string]interface{}{
		"status":    "healthy",
		"is_leader": ClusterManager.IsLeader(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}