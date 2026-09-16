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
	"testing"

	"go.uber.org/zap"
)

func resetIgnoredDevices(t *testing.T) {
	t.Helper()
	log = zap.NewNop()
	ignoredMu.Lock()
	IgnoredDevices = make(map[string]IgnoredDevice)
	ignoredMu.Unlock()
	ClusterBroadcaster = nil
	t.Cleanup(func() {
		ignoredMu.Lock()
		IgnoredDevices = make(map[string]IgnoredDevice)
		ignoredMu.Unlock()
		ClusterBroadcaster = nil
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

// Test_AddIgnoredDevice_IgnoresClientSuppliedEndpoint is a regression test
// for a credential-exfiltration vulnerability: AddIgnoredDevice must always
// derive Endpoint from Name/Model itself, never trust the Endpoint field on
// the passed-in IgnoredDevice, since that value could otherwise be combined
// with real Vault-backed credentials (via TestConn) and redirected to a
// destination of an attacker's choosing.
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

// Test_SetIgnoredDeviceLocal_IgnoresRemoteSuppliedEndpoint is a regression
// test covering the same vulnerability but for the gossip apply path: a
// compromised or misbehaving peer must not be able to inject an arbitrary
// Endpoint via a gossip add/merge message.
func Test_SetIgnoredDeviceLocal_IgnoresRemoteSuppliedEndpoint(t *testing.T) {
	resetIgnoredDevices(t)

	SetIgnoredDeviceLocal(IgnoredDevice{
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
// regression test for the credential-exfiltration vulnerability through the
// actual HTTP handler: a caller supplying an attacker-controlled Endpoint
// alongside a legitimate Name must have that Endpoint silently discarded and
// replaced with the canonical, derived one.
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
