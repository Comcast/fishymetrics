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

package gossip

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/comcast/fishymetrics/common"
	"github.com/hashicorp/memberlist"
	"go.uber.org/zap"
)

// Config holds the configuration needed to start the gossip Manager
type Config struct {
	NodeID        string
	BindAddr      string
	AdvertiseAddr string
	GossipPort    int

	DiscoveryMode DiscoveryMode
	ServiceName   string
	Namespace     string
	StaticPeers   []string

	// JoinRetries is the number of attempts made during Start() to discover
	// and join existing peers before falling back to single-node mode.
	// Defaults to 5 if unset.
	JoinRetries int
	// JoinRetryBackoff is the initial backoff duration between join attempts
	// during Start(); it doubles after each failed attempt. Defaults to 1s.
	JoinRetryBackoff time.Duration
	// ReconcileInterval controls how often a background goroutine re-checks
	// membership and attempts to (re)join peers if the node is isolated.
	// Defaults to 30s. Set to a negative value to disable reconciliation.
	ReconcileInterval time.Duration

	Logger *zap.Logger
}

// versionedRecord tracks the last known state of a single ignored host,
// including whether it was removed (tombstoned), so that memberlist's
// periodic full-state anti-entropy sync (LocalState/MergeRemoteState) can
// correctly resolve conflicts using last-write-wins semantics instead of
// resurrecting hosts that were intentionally removed by a peer that hasn't
// yet observed the removal.
type versionedRecord struct {
	Device  common.IgnoredDevice `json:"device"`
	Removed bool                 `json:"removed"`
	Version int64                `json:"version"`
}

// Manager wraps a memberlist.Memberlist instance and provides
// a simple API for broadcasting and applying ignored-host changes
// across the cluster via gossip.
type Manager struct {
	ml    *memberlist.Memberlist
	queue *memberlist.TransmitLimitedQueue
	log   *zap.Logger
	cfg   *Config

	stateMu     sync.Mutex
	state       map[string]versionedRecord
	lastVersion int64

	stopReconcile chan struct{}
}

// NewManager creates (but does not start) a new gossip Manager
func NewManager(cfg *Config) (*Manager, error) {
	if cfg.Logger == nil {
		cfg.Logger = zap.L()
	}
	if cfg.GossipPort == 0 {
		cfg.GossipPort = 7946
	}
	if cfg.NodeID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("failed to determine node id: %w", err)
		}
		cfg.NodeID = hostname
	}
	if cfg.BindAddr == "" {
		cfg.BindAddr = "0.0.0.0"
	}
	if cfg.JoinRetries == 0 {
		cfg.JoinRetries = 5
	}
	if cfg.JoinRetryBackoff == 0 {
		cfg.JoinRetryBackoff = time.Second
	}
	if cfg.ReconcileInterval == 0 {
		cfg.ReconcileInterval = 30 * time.Second
	}

	m := &Manager{
		log:           cfg.Logger,
		cfg:           cfg,
		state:         make(map[string]versionedRecord),
		stopReconcile: make(chan struct{}),
	}

	m.queue = &memberlist.TransmitLimitedQueue{
		NumNodes: func() int {
			if m.ml == nil {
				return 1
			}
			return m.ml.NumMembers()
		},
		RetransmitMult: 3,
	}

	mlConfig := memberlist.DefaultLANConfig()
	mlConfig.Name = cfg.NodeID
	mlConfig.BindAddr = cfg.BindAddr
	mlConfig.BindPort = cfg.GossipPort
	if cfg.AdvertiseAddr != "" {
		mlConfig.AdvertiseAddr = cfg.AdvertiseAddr
		mlConfig.AdvertisePort = cfg.GossipPort
	}
	mlConfig.Delegate = &MessageDelegate{manager: m, log: cfg.Logger}
	mlConfig.Events = &EventDelegate{log: cfg.Logger}
	mlConfig.LogOutput = newZapLogWriter(cfg.Logger)

	ml, err := memberlist.Create(mlConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create memberlist instance: %w", err)
	}
	m.ml = ml

	return m, nil
}

// Start attempts to discover and join existing cluster peers, retrying with
// backoff up to cfg.JoinRetries times to ride out transient issues (e.g. DNS
// propagation delay for a just-created headless Service at pod startup). If
// all attempts fail, the node starts as the sole member of the cluster and a
// background reconciliation loop (unless disabled) keeps retrying so the
// node can self-heal if peers become reachable later.
func (m *Manager) Start(ctx context.Context) error {
	joined := m.tryJoinWithRetry(ctx)
	if !joined {
		m.log.Warn("failed to join cluster after retries, starting as single node",
			zap.Int("attempts", m.cfg.JoinRetries))
	}

	if m.cfg.ReconcileInterval > 0 {
		go m.reconcileLoop(ctx)
	}

	return nil
}

// tryJoinWithRetry attempts discovery+join up to cfg.JoinRetries times with
// exponential backoff, returning true if it successfully joined at least one
// peer (or found no peers to join, which is a valid single-node start state
// only on the *last* attempt). Returns false if every attempt errored.
func (m *Manager) tryJoinWithRetry(ctx context.Context) bool {
	backoff := m.cfg.JoinRetryBackoff

	for attempt := 1; attempt <= m.cfg.JoinRetries; attempt++ {
		ok, noPeers := m.discoverAndJoin()
		if ok {
			return true
		}

		// If discovery succeeded but simply found no peers (e.g. this is
		// truly the first node up), there's nothing to retry yet - the
		// background reconciliation loop will pick up peers later if/when
		// they appear.
		if noPeers {
			return true
		}

		if attempt == m.cfg.JoinRetries {
			break
		}

		m.log.Info("retrying peer discovery/join", zap.Int("attempt", attempt), zap.Duration("backoff", backoff))

		select {
		case <-ctx.Done():
			return false
		case <-time.After(backoff):
		}
		backoff *= 2
	}

	return false
}

// discoverAndJoin performs a single discovery+join attempt.
// Returns (joined, noPeersFound).
func (m *Manager) discoverAndJoin() (bool, bool) {
	discoveryCfg := &DiscoveryConfig{
		Mode:        m.cfg.DiscoveryMode,
		ServiceName: m.cfg.ServiceName,
		Namespace:   m.cfg.Namespace,
		StaticPeers: m.cfg.StaticPeers,
		GossipPort:  m.cfg.GossipPort,
		Logger:      m.log,
	}
	discovery := NewClusterDiscovery(discoveryCfg)

	peers, err := discovery.DiscoverPeers()
	if err != nil {
		m.log.Warn("peer discovery failed", zap.Error(err))
		return false, false
	}

	if len(peers) == 0 {
		m.log.Info("no peers discovered")
		return false, true
	}

	// Don't try to join ourselves if discovery happens to return our own address
	numJoined, err := m.ml.Join(peers)
	if err != nil {
		m.log.Warn("failed to join any discovered peers", zap.Error(err), zap.Strings("peers", peers))
		return false, false
	}

	m.log.Info("joined cluster", zap.Int("peers_contacted", numJoined), zap.Int("member_count", m.ml.NumMembers()))
	return true, false
}

// reconcileLoop periodically checks membership and attempts to (re)join
// peers if this node appears isolated. This protects against transient
// failures (CoreDNS restarts, brief network partitions, etc.) causing
// permanent split-brain for the lifetime of the pod.
func (m *Manager) reconcileLoop(ctx context.Context) {
	ticker := time.NewTicker(m.cfg.ReconcileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopReconcile:
			return
		case <-ticker.C:
			if m.ml.NumMembers() > 1 {
				continue
			}

			m.log.Debug("node appears isolated, attempting to rejoin cluster")
			if ok, _ := m.discoverAndJoin(); ok && m.ml.NumMembers() > 1 {
				m.log.Info("recovered from isolation, rejoined cluster", zap.Int("member_count", m.ml.NumMembers()))
			}
		}
	}
}

// applyIfNewer updates the manager's tracked version state for the given
// host if the supplied record is newer than what's currently tracked (or
// nothing is tracked yet), returning true if the record was applied. This
// is the single source of truth used to resolve conflicting/out-of-order
// add and remove operations (whether arriving via NotifyMsg broadcast or
// MergeRemoteState anti-entropy sync) so that a stale "add" can never
// resurrect a host after a newer "remove" tombstone has been recorded, and
// vice versa.
func (m *Manager) applyIfNewer(host string, record versionedRecord) bool {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()

	existing, ok := m.state[host]
	if ok && record.Version <= existing.Version {
		return false
	}

	m.state[host] = record
	if record.Version > m.lastVersion {
		m.lastVersion = record.Version
	}
	return true
}

// encodeState returns a JSON snapshot of the manager's full versioned state
// (including tombstones for removed hosts), used by LocalState.
func (m *Manager) encodeState() ([]byte, error) {
	m.stateMu.Lock()
	snapshot := make(map[string]versionedRecord, len(m.state))
	for k, v := range m.state {
		snapshot[k] = v
	}
	m.stateMu.Unlock()

	return json.Marshal(snapshot)
}

// decodeState unmarshals a JSON state snapshot produced by encodeState.
func decodeState(buf []byte) (map[string]versionedRecord, error) {
	state := map[string]versionedRecord{}
	if len(buf) == 0 {
		return state, nil
	}
	if err := json.Unmarshal(buf, &state); err != nil {
		return nil, err
	}
	return state, nil
}

// Shutdown gracefully leaves the cluster and shuts down the local memberlist instance
func (m *Manager) Shutdown() error {
	close(m.stopReconcile)

	if m.ml == nil {
		return nil
	}

	leaveTimeout := 5 * time.Second
	if err := m.ml.Leave(leaveTimeout); err != nil {
		m.log.Warn("error leaving cluster", zap.Error(err))
	}

	return m.ml.Shutdown()
}

// AddIgnoredDevice updates local state and broadcasts the addition to the cluster.
// Implements common.Broadcaster.
func (m *Manager) BroadcastAdd(device common.IgnoredDevice) error {
	version := m.nextVersion()

	m.stateMu.Lock()
	m.state[device.Name] = versionedRecord{Device: device, Removed: false, Version: version}
	m.stateMu.Unlock()

	msg := &Message{
		Type:    MessageTypeAddIgnored,
		Device:  device,
		Version: version,
	}
	data, err := msg.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode add message: %w", err)
	}

	m.queue.QueueBroadcast(&broadcast{msg: data})
	return nil
}

// BroadcastRemove broadcasts the removal of a host to the cluster.
// Implements common.Broadcaster.
func (m *Manager) BroadcastRemove(host string) error {
	version := m.nextVersion()

	m.stateMu.Lock()
	m.state[host] = versionedRecord{Device: common.IgnoredDevice{Name: host}, Removed: true, Version: version}
	m.stateMu.Unlock()

	msg := &Message{
		Type:    MessageTypeRemoveIgnored,
		Host:    host,
		Version: version,
	}
	data, err := msg.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode remove message: %w", err)
	}

	m.queue.QueueBroadcast(&broadcast{msg: data})
	return nil
}

// nextVersion returns a strictly increasing logical clock value used to
// order ignored-host state changes for conflict resolution. Guards against
// two calls within the same nanosecond (or a system clock that moves
// backwards) ever producing a non-increasing value.
func (m *Manager) nextVersion() int64 {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()

	v := time.Now().UnixNano()
	if v <= m.lastVersion {
		v = m.lastVersion + 1
	}
	m.lastVersion = v
	return v
}

// MemberCount returns the number of members currently in the cluster
func (m *Manager) MemberCount() int {
	if m.ml == nil {
		return 0
	}
	return m.ml.NumMembers()
}

// ClusterMembers returns the list of current cluster members.
// Implements common.ClusterInfo.
func (m *Manager) ClusterMembers() []common.ClusterMember {
	if m.ml == nil {
		return nil
	}
	nodes := m.ml.Members()
	members := make([]common.ClusterMember, 0, len(nodes))
	for _, n := range nodes {
		members = append(members, common.ClusterMember{
			Name: n.Name,
			Addr: n.Address(),
		})
	}
	return members
}

// LocalNodeName returns the local node's name. Implements common.ClusterInfo.
func (m *Manager) LocalNodeName() string {
	if m.ml == nil {
		return ""
	}
	return m.ml.LocalNode().Name
}

// broadcast implements memberlist.Broadcast
type broadcast struct {
	msg []byte
}

func (b *broadcast) Invalidates(other memberlist.Broadcast) bool {
	return false
}

func (b *broadcast) Message() []byte {
	return b.msg
}

func (b *broadcast) Finished() {}
