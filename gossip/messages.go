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
	"encoding/json"

	"github.com/comcast/fishymetrics/common"
)

// MessageType represents the type of gossip message
type MessageType string

const (
	MessageTypeAddIgnored    MessageType = "add_ignored"
	MessageTypeRemoveIgnored MessageType = "remove_ignored"
)

// Message is a gossip broadcast message
type Message struct {
	Type    MessageType          `json:"type"`
	Device  common.IgnoredDevice `json:"device,omitempty"`
	Host    string               `json:"host,omitempty"` // Used for remove operations
	Version int64                `json:"version"`        // logical clock (UnixNano) used to resolve conflicting/out-of-order updates and prevent tombstoned removals from being resurrected by anti-entropy sync
}

// Encode marshals the message to JSON
func (m *Message) Encode() ([]byte, error) {
	return json.Marshal(m)
}

// DecodeMessage unmarshals a message from JSON
func DecodeMessage(data []byte) (*Message, error) {
	var msg Message
	err := json.Unmarshal(data, &msg)
	return &msg, err
}
