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
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
	"go.uber.org/zap"
)

type RaftNode struct {
	raft           *raft.Raft
	fsm            *FSM
	log            *zap.Logger
	dataDir        string
	bindAddr       string
	advertiseAddr  string
	nodeID         string
	bootstrapPeers []string
}

type Config struct {
	DataDir        string
	BindAddr       string
	AdvertiseAddr  string
	NodeID         string
	BootstrapPeers []string
	Logger         *zap.Logger
}

func NewRaftNode(config Config) (*RaftNode, error) {
	if config.Logger == nil {
		config.Logger = zap.L()
	}

	node := &RaftNode{
		log:            config.Logger,
		dataDir:        config.DataDir,
		bindAddr:       config.BindAddr,
		advertiseAddr:  config.AdvertiseAddr,
		nodeID:         config.NodeID,
		bootstrapPeers: config.BootstrapPeers,
	}

	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	return node, nil
}

func (r *RaftNode) Start() error {
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(r.nodeID)
	config.LogLevel = "INFO"

	addr, err := net.ResolveTCPAddr("tcp", r.bindAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve bind address: %w", err)
	}

	transport, err := raft.NewTCPTransport(r.bindAddr, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return fmt.Errorf("failed to create transport: %w", err)
	}

	// Create the snapshot store
	snapshotStore, err := raft.NewFileSnapshotStore(r.dataDir, 2, os.Stderr)
	if err != nil {
		return fmt.Errorf("failed to create snapshot store: %w", err)
	}

	// Create the log store and stable store
	logStore, err := raftboltdb.NewBoltStore(filepath.Join(r.dataDir, "raft.db"))
	if err != nil {
		return fmt.Errorf("failed to create bolt store: %w", err)
	}

	// Create FSM
	r.fsm = NewFSM(r.log)

	// Create the Raft instance
	ra, err := raft.NewRaft(config, r.fsm, logStore, logStore, snapshotStore, transport)
	if err != nil {
		return fmt.Errorf("failed to create raft: %w", err)
	}

	r.raft = ra

	// Bootstrap cluster if needed
	if len(r.bootstrapPeers) == 0 {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      raft.ServerID(r.nodeID),
					Address: raft.ServerAddress(r.advertiseAddr),
				},
			},
		}
		future := r.raft.BootstrapCluster(configuration)
		if err := future.Error(); err != nil {
			r.log.Warn("failed to bootstrap cluster", zap.Error(err))
		}
	}

	return nil
}

func (r *RaftNode) IsLeader() bool {
	return r.raft.State() == raft.Leader
}

func (r *RaftNode) GetLeader() string {
	_, leader := r.raft.LeaderWithID()
	return string(leader)
}

func (r *RaftNode) Apply(cmd interface{}, timeout time.Duration) error {
	if !r.IsLeader() {
		return fmt.Errorf("not the leader")
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}

	future := r.raft.Apply(data, timeout)
	if err := future.Error(); err != nil {
		return fmt.Errorf("failed to apply command: %w", err)
	}

	response := future.Response()
	if err, ok := response.(error); ok {
		return err
	}

	return nil
}

func (r *RaftNode) JoinCluster(nodeID, addr string) error {
	if !r.IsLeader() {
		return fmt.Errorf("not the leader")
	}

	configFuture := r.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		return fmt.Errorf("failed to get configuration: %w", err)
	}

	for _, srv := range configFuture.Configuration().Servers {
		if srv.ID == raft.ServerID(nodeID) || srv.Address == raft.ServerAddress(addr) {
			if srv.Address == raft.ServerAddress(addr) && srv.ID == raft.ServerID(nodeID) {
				r.log.Info("node already member of cluster, ignoring join request", 
					zap.String("node_id", nodeID), zap.String("addr", addr))
				return nil
			}

			future := r.raft.RemoveServer(srv.ID, 0, 0)
			if err := future.Error(); err != nil {
				return fmt.Errorf("error removing existing node: %w", err)
			}
		}
	}

	future := r.raft.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(addr), 0, 0)
	if err := future.Error(); err != nil {
		return fmt.Errorf("failed to add voter: %w", err)
	}

	r.log.Info("node joined cluster", zap.String("node_id", nodeID), zap.String("addr", addr))
	return nil
}

func (r *RaftNode) LeaveCluster(nodeID string) error {
	if !r.IsLeader() {
		return fmt.Errorf("not the leader")
	}

	future := r.raft.RemoveServer(raft.ServerID(nodeID), 0, 0)
	if err := future.Error(); err != nil {
		return fmt.Errorf("failed to remove server: %w", err)
	}

	r.log.Info("node left cluster", zap.String("node_id", nodeID))
	return nil
}

func (r *RaftNode) GetState() *SharedState {
	return r.fsm.GetState()
}

func (r *RaftNode) Shutdown() error {
	if r.raft != nil {
		future := r.raft.Shutdown()
		if err := future.Error(); err != nil {
			return fmt.Errorf("failed to shutdown raft: %w", err)
		}
	}
	return nil
}

func (r *RaftNode) Stats() map[string]string {
	if r.raft == nil {
		return map[string]string{}
	}
	return r.raft.Stats()
}

func (r *RaftNode) GetPeers() ([]raft.Server, error) {
	configFuture := r.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		return nil, fmt.Errorf("failed to get configuration: %w", err)
	}
	return configFuture.Configuration().Servers, nil
}