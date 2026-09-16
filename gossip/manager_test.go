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
	"testing"
	"time"

	"github.com/comcast/fishymetrics/common"
	"github.com/hashicorp/memberlist"
	"go.uber.org/zap"
)

// newTestManager creates a Manager bound to localhost on the given port,
// with no discovery configured (peers are joined manually in the test).
func newTestManager(t *testing.T, nodeID string, port int) *Manager {
	t.Helper()

	cfg := &Config{
		NodeID:        nodeID,
		BindAddr:      "127.0.0.1",
		AdvertiseAddr: "127.0.0.1",
		GossipPort:    port,
		DiscoveryMode: DiscoveryModeStatic,
		StaticPeers:   nil,
		Logger:        zap.NewNop(),
	}

	m, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("failed to create manager %s: %v", nodeID, err)
	}

	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("failed to start manager %s: %v", nodeID, err)
	}

	t.Cleanup(func() {
		_ = m.Shutdown()
	})

	return m
}

// TestMembershipConvergence verifies that memberlist instances discover and
// join each other correctly, forming a single cluster. This does not depend
// on common.IgnoredDevices since that state is process-global and would not
// realistically model separate pods within a single test process.
func TestMembershipConvergence(t *testing.T) {
	basePort := 27946
	node1 := newTestManager(t, "node1-membership", basePort)
	node2 := newTestManager(t, "node2-membership", basePort+1)
	node3 := newTestManager(t, "node3-membership", basePort+2)

	peer1 := fmt.Sprintf("127.0.0.1:%d", basePort)
	if _, err := node2.ml.Join([]string{peer1}); err != nil {
		t.Fatalf("node2 failed to join node1: %v", err)
	}
	if _, err := node3.ml.Join([]string{peer1}); err != nil {
		t.Fatalf("node3 failed to join node1: %v", err)
	}

	waitForMemberCount(t, node1, 3, 5*time.Second)
	waitForMemberCount(t, node2, 3, 5*time.Second)
	waitForMemberCount(t, node3, 3, 5*time.Second)

	// sanity check ClusterMembers()/LocalNodeName() used by /cluster/status
	if node1.LocalNodeName() != "node1-membership" {
		t.Errorf("expected local node name %q, got %q", "node1-membership", node1.LocalNodeName())
	}
	if len(node1.ClusterMembers()) != 3 {
		t.Errorf("expected 3 cluster members reported, got %d", len(node1.ClusterMembers()))
	}
}

// TestMessageEncodeDecode verifies gossip message round-trips through JSON encoding.
func TestMessageEncodeDecode(t *testing.T) {
	original := &Message{
		Type: MessageTypeAddIgnored,
		Device: common.IgnoredDevice{
			Name:              "host-1",
			Endpoint:          "https://host-1/redfish/v1/Chassis/",
			Model:             "TestModel",
			CredentialProfile: "default",
		},
	}

	data, err := original.Encode()
	if err != nil {
		t.Fatalf("failed to encode message: %v", err)
	}

	decoded, err := DecodeMessage(data)
	if err != nil {
		t.Fatalf("failed to decode message: %v", err)
	}

	if decoded.Type != original.Type || decoded.Device.Name != original.Device.Name {
		t.Errorf("decoded message does not match original: got %+v, want %+v", decoded, original)
	}
}

// TestNotifyMsgAppliesToLocalState verifies that the MessageDelegate correctly
// applies incoming add/remove gossip messages to the local common.IgnoredDevices state.
func TestNotifyMsgAppliesToLocalState(t *testing.T) {
	common.ClusterBroadcaster = nil
	defer func() {
		common.RemoveIgnoredDeviceHost("notify-test-host")
	}()

	m := &Manager{
		log: zap.NewNop(),
	}
	delegate := &MessageDelegate{
		manager: m,
		log:     zap.NewNop(),
	}

	addMsg := &Message{
		Type: MessageTypeAddIgnored,
		Device: common.IgnoredDevice{
			Name:              "notify-test-host",
			Endpoint:          "https://notify-test-host/redfish/v1/Chassis/",
			Model:             "TestModel",
			CredentialProfile: "default",
		},
		Version: 1,
	}
	data, err := addMsg.Encode()
	if err != nil {
		t.Fatalf("failed to encode add message: %v", err)
	}

	delegate.NotifyMsg(data)

	if !common.IsIgnored("notify-test-host") {
		t.Fatal("expected host to be marked ignored after NotifyMsg(add)")
	}

	removeMsg := &Message{
		Type:    MessageTypeRemoveIgnored,
		Host:    "notify-test-host",
		Version: 2,
	}
	data, err = removeMsg.Encode()
	if err != nil {
		t.Fatalf("failed to encode remove message: %v", err)
	}

	delegate.NotifyMsg(data)

	if common.IsIgnored("notify-test-host") {
		t.Fatal("expected host to no longer be ignored after NotifyMsg(remove)")
	}
}

// TestNotifyMsgIgnoresStaleMessages verifies that a stale (older or
// equal-version) message never overrides a more recent state change - this
// is the core protection against the anti-entropy resurrection bug where a
// peer that hasn't yet observed a removal re-introduces a host that was
// already removed elsewhere.
func TestNotifyMsgIgnoresStaleMessages(t *testing.T) {
	common.ClusterBroadcaster = nil
	defer func() {
		common.RemoveIgnoredDeviceHost("stale-test-host")
	}()

	m := &Manager{
		log: zap.NewNop(),
	}
	delegate := &MessageDelegate{manager: m, log: zap.NewNop()}

	device := common.IgnoredDevice{Name: "stale-test-host", Endpoint: "https://stale-test-host/redfish/v1/Chassis/"}

	// Apply a remove at version 10 (simulating the authoritative, most
	// recent, user-initiated removal).
	removeMsg := &Message{Type: MessageTypeRemoveIgnored, Host: "stale-test-host", Version: 10}
	data, _ := removeMsg.Encode()
	delegate.NotifyMsg(data)

	if common.IsIgnored("stale-test-host") {
		t.Fatal("expected host to be removed after version 10 remove")
	}

	// A stale "add" at an older version (e.g. delayed/duplicate delivery of
	// a message that predates the removal) must NOT resurrect the host.
	staleAddMsg := &Message{Type: MessageTypeAddIgnored, Device: device, Version: 5}
	data, _ = staleAddMsg.Encode()
	delegate.NotifyMsg(data)

	if common.IsIgnored("stale-test-host") {
		t.Fatal("stale add message (version 5) incorrectly resurrected a host removed at version 10")
	}

	// An add at a newer version than the removal SHOULD be applied (e.g. the
	// host legitimately failed credentials again after being cleared).
	newAddMsg := &Message{Type: MessageTypeAddIgnored, Device: device, Version: 20}
	data, _ = newAddMsg.Encode()
	delegate.NotifyMsg(data)

	if !common.IsIgnored("stale-test-host") {
		t.Fatal("expected newer add (version 20) to be applied after a version 10 removal")
	}
}

// TestMergeRemoteStateDoesNotResurrectRemovedHost reproduces the real-world
// bug report: node A adds a host, it replicates to node B; node A then
// removes the host, but node B's periodic anti-entropy push/pull
// (LocalState/MergeRemoteState) must not resurrect it on node A using B's
// stale (pre-removal) snapshot.
func TestMergeRemoteStateDoesNotResurrectRemovedHost(t *testing.T) {
	defer func() {
		common.ClusterBroadcaster = nil
		common.RemoveIgnoredDeviceHost("merge-test-host")
	}()

	nodeA := &Manager{log: zap.NewNop(), cfg: &Config{NodeID: "node-a"}}
	nodeA.queue = &memberlist.TransmitLimitedQueue{
		NumNodes:       func() int { return 1 },
		RetransmitMult: 3,
	}
	delegateA := &MessageDelegate{manager: nodeA, log: zap.NewNop()}

	device := common.IgnoredDevice{Name: "merge-test-host", Endpoint: "https://merge-test-host/redfish/v1/Chassis/"}

	// Node A adds the host then removes it - mirroring "add replicated,
	// then user clicks remove" - via the real production call path
	// (common.AddIgnoredDevice/RemoveIgnoredDeviceHost with
	// ClusterBroadcaster set to nodeA), so that local state and version
	// allocation happen exactly as they would in production, atomically.
	common.ClusterBroadcaster = nodeA
	common.AddIgnoredDevice(device)
	common.RemoveIgnoredDeviceHost(device.Name)

	if common.IsIgnored(device.Name) {
		t.Fatal("expected host to be removed on node A before merge")
	}

	// Node B never saw the removal - its snapshot still shows the host as
	// present, with an OLDER version than node A's removal.
	nodeAVersion := common.SnapshotIgnoredRecords()[device.Name].Version
	remoteSnapshot := map[string]common.IgnoredRecord{
		device.Name: {Device: device, Removed: false, Version: nodeAVersion - 1},
	}
	buf, err := json.Marshal(remoteSnapshot)
	if err != nil {
		t.Fatalf("failed to marshal remote snapshot: %v", err)
	}

	// Simulate memberlist's periodic anti-entropy sync delivering node B's
	// stale state to node A.
	delegateA.MergeRemoteState(buf, false)

	if common.IsIgnored(device.Name) {
		t.Fatal("anti-entropy merge incorrectly resurrected a host that was removed at a newer version")
	}
}

// TestMergeRemoteStateBreaksTiesDeterministically is a regression test for
// a P2 bug: Version is only monotonic per-node, so two nodes can produce
// the same Version for conflicting updates. Without a tie-breaker, this
// would be permanent split-brain even across anti-entropy sync.
func TestMergeRemoteStateBreaksTiesDeterministically(t *testing.T) {
	defer func() {
		common.RemoveIgnoredDeviceHost("tie-test-host")
	}()

	device := common.IgnoredDevice{Name: "tie-test-host", Endpoint: "https://tie-test-host/redfish/v1/Chassis/"}
	const sharedVersion = 42

	nodeA := &Manager{log: zap.NewNop(), cfg: &Config{NodeID: "node-a"}}
	delegateA := &MessageDelegate{manager: nodeA, log: zap.NewNop()}

	// Node A believes it added the host at sharedVersion.
	if !common.ApplyRemoteRecord(device.Name, device, false, sharedVersion, "node-a") {
		t.Fatal("expected node-a's record to apply")
	}

	// Node B independently produced the SAME version but removed the host.
	// Deliver node B's snapshot to node A via anti-entropy sync.
	remoteSnapshot := map[string]common.IgnoredRecord{
		device.Name: {Device: device, Removed: true, Version: sharedVersion, NodeID: "node-b"},
	}
	buf, err := json.Marshal(remoteSnapshot)
	if err != nil {
		t.Fatalf("failed to marshal remote snapshot: %v", err)
	}
	delegateA.MergeRemoteState(buf, false)

	// "node-b" > "node-a" lexicographically, so node B's remove must win
	// the tie deterministically on node A too.
	if common.IsIgnored(device.Name) {
		t.Fatal("expected node-b's remove to win the tie over node-a's add during anti-entropy merge")
	}
}

// TestBroadcastQueue verifies BroadcastAdd/BroadcastRemove enqueue messages
// that can be drained via GetBroadcasts, as memberlist does internally.
func TestBroadcastQueue(t *testing.T) {
	m := newTestManager(t, "node-broadcast", 27960)

	device := common.IgnoredDevice{
		Name:              "broadcast-host",
		Endpoint:          "https://broadcast-host/redfish/v1/Chassis/",
		Model:             "TestModel",
		CredentialProfile: "default",
	}

	if err := m.BroadcastAdd(device, 1); err != nil {
		t.Fatalf("failed to broadcast add: %v", err)
	}

	broadcasts := m.queue.GetBroadcasts(0, 1024)
	if len(broadcasts) == 0 {
		t.Fatal("expected at least one queued broadcast after BroadcastAdd")
	}

	msg, err := DecodeMessage(broadcasts[0])
	if err != nil {
		t.Fatalf("failed to decode queued broadcast: %v", err)
	}
	if msg.Type != MessageTypeAddIgnored || msg.Device.Name != "broadcast-host" {
		t.Errorf("unexpected queued message: %+v", msg)
	}
}

func waitForMemberCount(t *testing.T, m *Manager, expected int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if m.MemberCount() >= expected {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for member count >= %d, got %d", expected, m.MemberCount())
}

// TestStartRetriesOnTransientDiscoveryFailure verifies that Start() retries
// peer discovery/join rather than giving up permanently after a single
// failure - this reproduces the real-world bug where a headless Service's
// DNS record isn't propagated yet at the exact moment a pod boots.
func TestStartRetriesOnTransientDiscoveryFailure(t *testing.T) {
	basePort := 27970

	// node1 starts first and will not be listening on the gossip port
	// for the first ~1.5s, simulating "peer not ready yet" / DNS not
	// propagated yet.
	peer1Addr := fmt.Sprintf("127.0.0.1:%d", basePort)

	cfg2 := &Config{
		NodeID:           "node2-retry",
		BindAddr:         "127.0.0.1",
		AdvertiseAddr:    "127.0.0.1",
		GossipPort:       basePort + 1,
		DiscoveryMode:    DiscoveryModeStatic,
		StaticPeers:      []string{peer1Addr},
		JoinRetries:      6,
		JoinRetryBackoff: 300 * time.Millisecond,
		Logger:           zap.NewNop(),
	}
	node2, err := NewManager(cfg2)
	if err != nil {
		t.Fatalf("failed to create node2: %v", err)
	}
	t.Cleanup(func() { _ = node2.Shutdown() })

	startDone := make(chan struct{})
	go func() {
		_ = node2.Start(context.Background())
		close(startDone)
	}()

	// Delay bringing node1 online to force node2's first couple of join
	// attempts to fail, exercising the retry/backoff path.
	time.Sleep(700 * time.Millisecond)

	node1 := newTestManager(t, "node1-retry", basePort)
	_ = node1

	select {
	case <-startDone:
	case <-time.After(10 * time.Second):
		t.Fatal("Start() did not return in time")
	}

	waitForMemberCount(t, node2, 2, 5*time.Second)
}

// TestReconcileLoopRecoversFromIsolation verifies that the background
// reconciliation loop rejoins the cluster after an initial join failure,
// once a peer becomes available, without requiring a process restart.
func TestReconcileLoopRecoversFromIsolation(t *testing.T) {
	basePort := 27980
	peer1Addr := fmt.Sprintf("127.0.0.1:%d", basePort)

	cfg2 := &Config{
		NodeID:            "node2-reconcile",
		BindAddr:          "127.0.0.1",
		AdvertiseAddr:     "127.0.0.1",
		GossipPort:        basePort + 1,
		DiscoveryMode:     DiscoveryModeStatic,
		StaticPeers:       []string{peer1Addr},
		JoinRetries:       1, // fail fast so we rely on reconciliation, not startup retry
		JoinRetryBackoff:  100 * time.Millisecond,
		ReconcileInterval: 500 * time.Millisecond,
		Logger:            zap.NewNop(),
	}
	node2, err := NewManager(cfg2)
	if err != nil {
		t.Fatalf("failed to create node2: %v", err)
	}
	t.Cleanup(func() { _ = node2.Shutdown() })

	if err := node2.Start(context.Background()); err != nil {
		t.Fatalf("failed to start node2: %v", err)
	}

	// node2 should have started isolated since node1 isn't up yet.
	if node2.MemberCount() != 1 {
		t.Fatalf("expected node2 to start isolated, got member count %d", node2.MemberCount())
	}

	// Now bring node1 online; the reconciliation loop should discover and
	// join it within a couple of ticks.
	newTestManager(t, "node1-reconcile", basePort)

	waitForMemberCount(t, node2, 2, 5*time.Second)
}
