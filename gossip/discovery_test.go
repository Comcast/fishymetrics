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
	"net"
	"testing"
)

// Test_FormatPeerAddr_BracketsIPv6 is a regression test: an IPv6 address
// formatted with a plain "%s:%d" (e.g. "::1:7946") is unparseable/wrong -
// memberlist requires the bracketed "[::1]:7946" form. IPv4 must be
// unaffected by the change.
func Test_FormatPeerAddr_BracketsIPv6(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		port int
		want string
	}{
		{"IPv4", "10.0.0.5", 7946, "10.0.0.5:7946"},
		{"IPv6 loopback", "::1", 7946, "[::1]:7946"},
		{"IPv6 full", "2001:db8::1", 28997, "[2001:db8::1]:28997"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse test IP %q", tt.ip)
			}
			got := formatPeerAddr(ip, tt.port)
			if got != tt.want {
				t.Errorf("formatPeerAddr(%q, %d) = %q, want %q", tt.ip, tt.port, got, tt.want)
			}

			// The result must also be a valid address memberlist can
			// resolve via net.SplitHostPort - this is what actually failed
			// in the bug report ("::1:28997" is not a valid host:port).
			host, port, err := net.SplitHostPort(got)
			if err != nil {
				t.Fatalf("formatPeerAddr produced an unparseable address %q: %v", got, err)
			}
			if net.ParseIP(host) == nil {
				t.Errorf("SplitHostPort extracted host %q, which is not a valid IP", host)
			}
			if port == "" {
				t.Error("expected a non-empty port")
			}
		})
	}
}
