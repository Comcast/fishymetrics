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

package raft

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/comcast/fishymetrics/common"
	"github.com/hashicorp/raft"
	"go.uber.org/zap"
)

type CommandType string

const (
	CommandSetVaultToken     CommandType = "set_vault_token"
	CommandRemoveVaultToken  CommandType = "remove_vault_token"
	CommandAddIgnoredDevice  CommandType = "add_ignored_device"
	CommandRemoveIgnoredDevice CommandType = "remove_ignored_device"
	CommandUpdateSharedData  CommandType = "update_shared_data"
)

type Command struct {
	Type CommandType     `json:"type"`
	Data json.RawMessage `json:"data"`
}

type VaultTokenData struct {
	Token      string `json:"token"`
	ExpiresAt  int64  `json:"expires_at"`
	RoleID     string `json:"role_id"`
	SecretID   string `json:"secret_id"`
}

type SharedState struct {
	VaultToken     *VaultTokenData                  `json:"vault_token"`
	IgnoredDevices map[string]common.IgnoredDevice `json:"ignored_devices"`
	CustomData     map[string]interface{}          `json:"custom_data"`
}

type FSM struct {
	mu    sync.RWMutex
	state *SharedState
	log   *zap.Logger
}

func NewFSM(logger *zap.Logger) *FSM {
	return &FSM{
		state: &SharedState{
			IgnoredDevices: make(map[string]common.IgnoredDevice),
			CustomData:     make(map[string]interface{}),
		},
		log: logger,
	}
}

func (f *FSM) Apply(l *raft.Log) interface{} {
	var cmd Command
	if err := json.Unmarshal(l.Data, &cmd); err != nil {
		f.log.Error("failed to unmarshal command", zap.Error(err))
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	switch cmd.Type {
	case CommandSetVaultToken:
		var tokenData VaultTokenData
		if err := json.Unmarshal(cmd.Data, &tokenData); err != nil {
			f.log.Error("failed to unmarshal vault token data", zap.Error(err))
			return err
		}
		f.state.VaultToken = &tokenData
		f.log.Info("vault token updated in shared state")

	case CommandRemoveVaultToken:
		f.state.VaultToken = nil
		f.log.Info("vault token removed from shared state")

	case CommandAddIgnoredDevice:
		var device common.IgnoredDevice
		if err := json.Unmarshal(cmd.Data, &device); err != nil {
			f.log.Error("failed to unmarshal ignored device", zap.Error(err))
			return err
		}
		if f.state.IgnoredDevices == nil {
			f.state.IgnoredDevices = make(map[string]common.IgnoredDevice)
		}
		f.state.IgnoredDevices[device.Name] = device
		f.log.Info("ignored device added", zap.String("device", device.Name))

	case CommandRemoveIgnoredDevice:
		var deviceName string
		if err := json.Unmarshal(cmd.Data, &deviceName); err != nil {
			f.log.Error("failed to unmarshal device name", zap.Error(err))
			return err
		}
		delete(f.state.IgnoredDevices, deviceName)
		f.log.Info("ignored device removed", zap.String("device", deviceName))

	case CommandUpdateSharedData:
		var data map[string]interface{}
		if err := json.Unmarshal(cmd.Data, &data); err != nil {
			f.log.Error("failed to unmarshal shared data", zap.Error(err))
			return err
		}
		for k, v := range data {
			f.state.CustomData[k] = v
		}
		f.log.Info("shared data updated")

	default:
		err := fmt.Errorf("unknown command type: %s", cmd.Type)
		f.log.Error("unknown command type", zap.String("type", string(cmd.Type)))
		return err
	}

	return nil
}

func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Create a deep copy of the state
	stateCopy := &SharedState{
		VaultToken:     f.state.VaultToken,
		IgnoredDevices: make(map[string]common.IgnoredDevice),
		CustomData:     make(map[string]interface{}),
	}

	for k, v := range f.state.IgnoredDevices {
		stateCopy.IgnoredDevices[k] = v
	}

	for k, v := range f.state.CustomData {
		stateCopy.CustomData[k] = v
	}

	return &FSMSnapshot{state: stateCopy, log: f.log}, nil
}

func (f *FSM) Restore(snapshot io.ReadCloser) error {
	defer snapshot.Close()

	var state SharedState
	decoder := json.NewDecoder(snapshot)
	if err := decoder.Decode(&state); err != nil {
		return fmt.Errorf("failed to restore snapshot: %w", err)
	}

	f.mu.Lock()
	f.state = &state
	f.mu.Unlock()

	// Sync the global IgnoredDevices map with the restored state
	common.IgnoredDevices = state.IgnoredDevices

	f.log.Info("state restored from snapshot")
	return nil
}

func (f *FSM) GetState() *SharedState {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Return a copy to prevent external modifications
	stateCopy := &SharedState{
		VaultToken:     f.state.VaultToken,
		IgnoredDevices: make(map[string]common.IgnoredDevice),
		CustomData:     make(map[string]interface{}),
	}

	for k, v := range f.state.IgnoredDevices {
		stateCopy.IgnoredDevices[k] = v
	}

	for k, v := range f.state.CustomData {
		stateCopy.CustomData[k] = v
	}

	return stateCopy
}

type FSMSnapshot struct {
	state *SharedState
	log   *zap.Logger
}

func (s *FSMSnapshot) Persist(sink raft.SnapshotSink) error {
	encoder := json.NewEncoder(sink)
	if err := encoder.Encode(s.state); err != nil {
		sink.Cancel()
		return fmt.Errorf("failed to persist snapshot: %w", err)
	}

	if err := sink.Close(); err != nil {
		return fmt.Errorf("failed to close sink: %w", err)
	}

	s.log.Info("snapshot persisted")
	return nil
}

func (s *FSMSnapshot) Release() {
	// Nothing to release
}