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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/comcast/fishymetrics/common"
	fishy_vault "github.com/comcast/fishymetrics/vault"
	"go.uber.org/zap"
)

type ClusterManager struct {
	raftNode         *RaftNode
	discovery        *ClusterDiscovery
	vault            *fishy_vault.Vault
	log              *zap.Logger
	mu               sync.RWMutex
	vaultTokenRenew  chan bool
	stopCh           chan struct{}
	wg               sync.WaitGroup
}

type ClusterConfig struct {
	NodeID          string
	DataDir         string
	BindAddr        string
	AdvertiseAddr   string
	RaftPort        int
	DiscoveryPort   int
	DiscoveryMode   DiscoveryMode
	ServiceName     string
	Namespace       string
	StaticPeers     []string
	Logger          *zap.Logger
}

func NewClusterManager(config ClusterConfig, vault *fishy_vault.Vault) (*ClusterManager, error) {
	if config.Logger == nil {
		config.Logger = zap.L()
	}

	if config.NodeID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("failed to get hostname: %w", err)
		}
		config.NodeID = hostname
	}

	if config.DataDir == "" {
		config.DataDir = "/var/lib/fishymetrics/raft"
	}

	raftConfig := Config{
		DataDir:       config.DataDir,
		BindAddr:      fmt.Sprintf("%s:%d", config.BindAddr, config.RaftPort),
		AdvertiseAddr: fmt.Sprintf("%s:%d", config.AdvertiseAddr, config.RaftPort),
		NodeID:        config.NodeID,
		Logger:        config.Logger,
	}

	raftNode, err := NewRaftNode(raftConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create raft node: %w", err)
	}

	discoveryConfig := &DiscoveryConfig{
		Mode:          config.DiscoveryMode,
		ServiceName:   config.ServiceName,
		Namespace:     config.Namespace,
		StaticPeers:   config.StaticPeers,
		RaftPort:      config.RaftPort,
		DiscoveryPort: config.DiscoveryPort,
		Logger:        config.Logger,
	}

	discovery := NewClusterDiscovery(discoveryConfig)

	cm := &ClusterManager{
		raftNode:        raftNode,
		discovery:       discovery,
		vault:           vault,
		log:             config.Logger,
		vaultTokenRenew: make(chan bool, 1),
		stopCh:          make(chan struct{}),
	}

	return cm, nil
}

func (cm *ClusterManager) Start(ctx context.Context) error {
	// Start Raft
	if err := cm.raftNode.Start(); err != nil {
		return fmt.Errorf("failed to start raft: %w", err)
	}

	// Start discovery server
	cm.discovery.StartDiscoveryServer(cm.raftNode)

	// Try to join existing cluster
	if err := cm.discovery.JoinCluster(ctx, cm.raftNode); err != nil {
		cm.log.Warn("failed to join cluster, continuing as standalone", zap.Error(err))
	}

	// Start background tasks
	cm.wg.Add(1)
	go cm.syncIgnoredDevices()

	if cm.vault != nil {
		cm.wg.Add(1)
		go cm.manageVaultToken(ctx)
	}

	cm.log.Info("cluster manager started", 
		zap.String("node_id", cm.raftNode.nodeID),
		zap.Bool("is_leader", cm.raftNode.IsLeader()))

	return nil
}

func (cm *ClusterManager) Stop() error {
	close(cm.stopCh)
	cm.wg.Wait()
	return cm.raftNode.Shutdown()
}

func (cm *ClusterManager) IsLeader() bool {
	return cm.raftNode.IsLeader()
}

func (cm *ClusterManager) GetLeader() string {
	return cm.raftNode.GetLeader()
}

func (cm *ClusterManager) GetState() *SharedState {
	return cm.raftNode.GetState()
}

func (cm *ClusterManager) ApplyCommand(cmd Command, timeout time.Duration) error {
	return cm.raftNode.Apply(cmd, timeout)
}

func (cm *ClusterManager) AddIgnoredDevice(device common.IgnoredDevice) error {
	data, err := json.Marshal(device)
	if err != nil {
		return fmt.Errorf("failed to marshal device: %w", err)
	}

	cmd := Command{
		Type: CommandAddIgnoredDevice,
		Data: data,
	}

	return cm.ApplyCommand(cmd, 5*time.Second)
}

func (cm *ClusterManager) RemoveIgnoredDevice(deviceName string) error {
	data, err := json.Marshal(deviceName)
	if err != nil {
		return fmt.Errorf("failed to marshal device name: %w", err)
	}

	cmd := Command{
		Type: CommandRemoveIgnoredDevice,
		Data: data,
	}

	return cm.ApplyCommand(cmd, 5*time.Second)
}

func (cm *ClusterManager) UpdateVaultToken(token string, expiresAt int64) error {
	tokenData := VaultTokenData{
		Token:     token,
		ExpiresAt: expiresAt,
		RoleID:    cm.vault.Parameters.ApproleRoleID,
		SecretID:  cm.vault.Parameters.ApproleSecretID,
	}

	data, err := json.Marshal(tokenData)
	if err != nil {
		return fmt.Errorf("failed to marshal token data: %w", err)
	}

	cmd := Command{
		Type: CommandSetVaultToken,
		Data: data,
	}

	return cm.ApplyCommand(cmd, 5*time.Second)
}

func (cm *ClusterManager) syncIgnoredDevices() {
	defer cm.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-cm.stopCh:
			return
		case <-ticker.C:
			state := cm.GetState()
			if state != nil && state.IgnoredDevices != nil {
				// Sync the global IgnoredDevices map with cluster state
				common.IgnoredDevices = state.IgnoredDevices
			}
		}
	}
}

func (cm *ClusterManager) manageVaultToken(ctx context.Context) {
	defer cm.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-cm.stopCh:
			return
		case <-ticker.C:
			if !cm.IsLeader() {
				// Only leader manages vault token
				continue
			}

			state := cm.GetState()
			if state == nil || state.VaultToken == nil {
				// No token in state, need to login
				cm.log.Info("no vault token in cluster state, attempting login")
				if err := cm.loginToVault(ctx); err != nil {
					cm.log.Error("failed to login to vault", zap.Error(err))
				}
				continue
			}

			// Check if token is about to expire
			now := time.Now().Unix()
			if state.VaultToken.ExpiresAt-now < 300 { // Renew if less than 5 minutes left
				cm.log.Info("vault token expiring soon, renewing")
				if err := cm.renewVaultToken(ctx); err != nil {
					cm.log.Error("failed to renew vault token", zap.Error(err))
					// Try to login again
					if err := cm.loginToVault(ctx); err != nil {
						cm.log.Error("failed to re-login to vault", zap.Error(err))
					}
				}
			}
		}
	}
}

func (cm *ClusterManager) loginToVault(ctx context.Context) error {
	if cm.vault == nil {
		return fmt.Errorf("vault client not configured")
	}

	// This would normally use the vault login method
	// For now, we'll simulate it
	token := "vault-token-" + cm.raftNode.nodeID
	expiresAt := time.Now().Add(1 * time.Hour).Unix()

	return cm.UpdateVaultToken(token, expiresAt)
}

func (cm *ClusterManager) renewVaultToken(ctx context.Context) error {
	state := cm.GetState()
	if state == nil || state.VaultToken == nil {
		return fmt.Errorf("no token to renew")
	}

	// Extend the expiration
	newExpiresAt := time.Now().Add(1 * time.Hour).Unix()
	return cm.UpdateVaultToken(state.VaultToken.Token, newExpiresAt)
}

func (cm *ClusterManager) GetVaultToken() string {
	state := cm.GetState()
	if state != nil && state.VaultToken != nil {
		return state.VaultToken.Token
	}
	return ""
}

// GetToken implements TokenProvider interface
func (cm *ClusterManager) GetToken() string {
	return cm.GetVaultToken()
}

// UpdateToken implements TokenProvider interface
func (cm *ClusterManager) UpdateToken(token string, expiresAt int64) error {
	return cm.UpdateVaultToken(token, expiresAt)
}