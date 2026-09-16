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
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

func resetIgnoredDevices(t *testing.T) {
	t.Helper()
	log = zap.NewNop()
	ignoredMu.Lock()
	records = make(map[string]IgnoredRecord)
	lastVersion = 0
	ignoredMu.Unlock()
	ClusterBroadcaster = nil
	t.Cleanup(func() {
		ignoredMu.Lock()
		records = make(map[string]IgnoredRecord)
		lastVersion = 0
		ignoredMu.Unlock()
		ClusterBroadcaster = nil
	})
}

// recordingBroadcaster observes what version/operation was last broadcast,
// so tests can assert local state never diverges from it. A small sleep
// widens the race window that used to exist before applyLocked/ApplyRemoteRecord.
type recordingBroadcaster struct {
	mu          sync.Mutex
	lastVersion int64
	lastRemoved bool
}

func (r *recordingBroadcaster) BroadcastAdd(device IgnoredDevice, version int64) error {
	time.Sleep(time.Millisecond)
	r.mu.Lock()
	defer r.mu.Unlock()
	if version > r.lastVersion {
		r.lastVersion = version
		r.lastRemoved = false
	}
	return nil
}

func (r *recordingBroadcaster) BroadcastRemove(hostname string, version int64) error {
	time.Sleep(time.Millisecond)
	r.mu.Lock()
	defer r.mu.Unlock()
	if version > r.lastVersion {
		r.lastVersion = version
		r.lastRemoved = true
	}
	return nil
}

// Test_ConcurrentAddRemove_LocalStateMatchesBroadcastVersion is a regression
// test for a P1 race where local map mutation and versioned broadcast were
// separate, independently-locked steps, letting local state disagree with
// the version broadcast to peers under concurrent Add/Remove of the same
// host. Now both happen atomically in applyLocked, so this always holds.
func Test_ConcurrentAddRemove_LocalStateMatchesBroadcastVersion(t *testing.T) {
	resetIgnoredDevices(t)

	rb := &recordingBroadcaster{}
	ClusterBroadcaster = rb

	device := IgnoredDevice{Name: "race-host", Model: "iLO5", CredentialProfile: "default"}

	const iterations = 50
	var wg sync.WaitGroup
	wg.Add(iterations * 2)
	for i := 0; i < iterations; i++ {
		go func() {
			defer wg.Done()
			AddIgnoredDevice(device)
		}()
		go func() {
			defer wg.Done()
			RemoveIgnoredDeviceHost(device.Name)
		}()
	}
	wg.Wait()

	rb.mu.Lock()
	wantIgnored := !rb.lastRemoved
	rb.mu.Unlock()

	gotIgnored := IsIgnored(device.Name)
	if gotIgnored != wantIgnored {
		t.Fatalf("local state (ignored=%v) is inconsistent with the highest-version broadcast operation (ignored=%v) - "+
			"local state and replicated/broadcast state diverged", gotIgnored, wantIgnored)
	}
}

// Test_RecordWins_TieBreaksOnNodeID verifies the (Version, NodeID) total
// order used by ApplyRemoteRecord to resolve equal-version conflicts.
func Test_RecordWins_TieBreaksOnNodeID(t *testing.T) {
	existing := IgnoredRecord{Version: 5, NodeID: "node-a"}

	if recordWins(5, "node-a", existing) {
		t.Error("identical (version, nodeID) should not win")
	}
	if !recordWins(5, "node-b", existing) {
		t.Error("expected node-b to win tie over node-a")
	}
	if recordWins(5, "node-", existing) {
		t.Error("expected lexicographically smaller NodeID to lose the tie")
	}
	if !recordWins(6, "node-a", existing) {
		t.Error("strictly greater version should win regardless of NodeID")
	}
	if recordWins(4, "node-z", existing) {
		t.Error("strictly smaller version should lose regardless of NodeID")
	}
}

// Test_ApplyRemoteRecord_DeterministicTieBreakAcrossNodes is a regression
// test for a P2 bug: two nodes can independently produce the same Version
// for conflicting updates. Without a tie-breaker, both would reject each
// other's record forever - permanent split-brain.
func Test_ApplyRemoteRecord_DeterministicTieBreakAcrossNodes(t *testing.T) {
	resetIgnoredDevices(t)

	device := IgnoredDevice{Name: "split-brain-host", Model: "iLO5"}
	const sharedVersion = 12345

	if !ApplyRemoteRecord(device.Name, device, false, sharedVersion, "node-a") {
		t.Fatal("expected first record to be applied")
	}

	// node-b independently produced the SAME version but removed the host.
	if !ApplyRemoteRecord(device.Name, device, true, sharedVersion, "node-b") {
		t.Fatal("expected node-b's record to win the tie over node-a")
	}
	if IsIgnored(device.Name) {
		t.Fatal("expected host to be removed after node-b's tie-winning remove")
	}

	// Re-applying node-a's losing record must still be rejected.
	if ApplyRemoteRecord(device.Name, device, false, sharedVersion, "node-a") {
		t.Fatal("expected node-a's record to remain the loser of the tie even when reapplied")
	}
}

// Test_ApplyRemoteRecord_TieBreakIsOrderIndependent proves the tie-break is
// commutative: either arrival order must converge on the same winner.
func Test_ApplyRemoteRecord_TieBreakIsOrderIndependent(t *testing.T) {
	device := IgnoredDevice{Name: "order-independent-host", Model: "iLO5"}
	const sharedVersion = 99

	t.Run("node-a then node-b", func(t *testing.T) {
		resetIgnoredDevices(t)
		ApplyRemoteRecord(device.Name, device, false, sharedVersion, "node-a")
		ApplyRemoteRecord(device.Name, device, true, sharedVersion, "node-b")
		if IsIgnored(device.Name) {
			t.Fatal("expected node-b to win regardless of arrival order")
		}
	})

	t.Run("node-b then node-a", func(t *testing.T) {
		resetIgnoredDevices(t)
		ApplyRemoteRecord(device.Name, device, true, sharedVersion, "node-b")
		ApplyRemoteRecord(device.Name, device, false, sharedVersion, "node-a")
		if IsIgnored(device.Name) {
			t.Fatal("expected node-b to still win regardless of arrival order")
		}
	})
}

func Test_BuildIgnoredDeviceEndpoint(t *testing.T) {
	tests := []struct {
		name  string
		host  string
		model string
		want  string
	}{
		{"default/redfish model", "host-1", "iLO5", "https://host-1/redfish/v1/Chassis/"},
		{"empty model defaults to redfish", "host-2", "", "https://host-2/redfish/v1/Chassis/"},
		{"moonshot model", "host-3", "Moonshot", "https://host-3/rest/v1/chassis/1"},
		{"trims whitespace in name", "  host-4  ", "iLO5", "https://host-4/redfish/v1/Chassis/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildIgnoredDeviceEndpoint(tt.host, tt.model)
			if got != tt.want {
				t.Errorf("BuildIgnoredDeviceEndpoint(%q, %q) = %q, want %q", tt.host, tt.model, got, tt.want)
			}
		})
	}
}

// Test_AddIgnoredDevice_IgnoresClientSuppliedEndpoint is a regression test:
// AddIgnoredDevice must always derive Endpoint from Name/Model, never trust
// the caller's Endpoint, since that could redirect real Vault credentials
// (via TestConn) to an attacker-controlled destination.
func Test_AddIgnoredDevice_IgnoresClientSuppliedEndpoint(t *testing.T) {
	resetIgnoredDevices(t)

	AddIgnoredDevice(IgnoredDevice{
		Name:              "real-host",
		Endpoint:          "http://attacker.example.com/collect",
		Model:             "iLO5",
		CredentialProfile: "default",
	})

	stored, ok := GetIgnoredDevice("real-host")
	if !ok {
		t.Fatal("expected device to be stored")
	}
	want := "https://real-host/redfish/v1/Chassis/"
	if stored.Endpoint != want {
		t.Fatalf("expected Endpoint to be re-derived to %q, got %q (attacker-controlled endpoint was not overridden)", want, stored.Endpoint)
	}
}

// Test_ApplyRemoteRecord_IgnoresRemoteSuppliedEndpoint covers the same
// vulnerability for the gossip apply path: a compromised peer must not be
// able to inject an arbitrary Endpoint.
func Test_ApplyRemoteRecord_IgnoresRemoteSuppliedEndpoint(t *testing.T) {
	resetIgnoredDevices(t)

	applied := ApplyRemoteRecord("real-host", IgnoredDevice{
		Name:              "real-host",
		Endpoint:          "http://attacker.example.com/collect",
		Model:             "iLO5",
		CredentialProfile: "default",
	}, false, 1, "peer-1")
	if !applied {
		t.Fatal("expected first record for a host to be applied")
	}

	stored, ok := GetIgnoredDevice("real-host")
	if !ok {
		t.Fatal("expected device to be stored")
	}
	want := "https://real-host/redfish/v1/Chassis/"
	if stored.Endpoint != want {
		t.Fatalf("expected Endpoint to be re-derived to %q, got %q (attacker-controlled endpoint was not overridden)", want, stored.Endpoint)
	}
}

// Test_GossipAwareAddHost_RejectsEmptyName ensures the HTTP handler validates
// input rather than allowing a device to be stored under an empty key.
func Test_GossipAwareAddHost_RejectsEmptyName(t *testing.T) {
	resetIgnoredDevices(t)

	body, _ := json.Marshal(IgnoredDevice{Name: "  ", Endpoint: "https://whatever/", Model: "iLO5"})
	req := httptest.NewRequest(http.MethodPost, "/ignored/add", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	GossipAwareAddHost(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for empty Name, got %d", rec.Code)
	}
}

// Test_GossipAwareAddHost_IgnoresClientSuppliedEndpoint is an end-to-end
// regression test: a caller-supplied Endpoint must be discarded and
// replaced with the derived one.
func Test_GossipAwareAddHost_IgnoresClientSuppliedEndpoint(t *testing.T) {
	resetIgnoredDevices(t)

	body, _ := json.Marshal(IgnoredDevice{
		Name:              "real-host",
		Endpoint:          "http://attacker.example.com/collect",
		Model:             "iLO5",
		CredentialProfile: "default",
	})
	req := httptest.NewRequest(http.MethodPost, "/ignored/add", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	GossipAwareAddHost(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	stored, ok := GetIgnoredDevice("real-host")
	if !ok {
		t.Fatal("expected device to be stored")
	}
	want := "https://real-host/redfish/v1/Chassis/"
	if stored.Endpoint != want {
		t.Fatalf("expected Endpoint to be re-derived to %q, got %q (attacker-controlled endpoint was not overridden)", want, stored.Endpoint)
	}
}
