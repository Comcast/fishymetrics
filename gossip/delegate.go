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
	"github.com/hashicorp/memberlist"
	"go.uber.org/zap"
)

// MessageDelegate implements memberlist.Delegate for message handling
type MessageDelegate struct {
	manager *Manager
	log     *zap.Logger
}

// NodeMeta implements memberlist.Delegate.NodeMeta
// We don't attach any per-node metadata beyond the default.
func (d *MessageDelegate) NodeMeta(limit int) []byte {
	return []byte{}
}

// NotifyMsg implements memberlist.Delegate.NotifyMsg
// This is called when a message is received from the network
func (d *MessageDelegate) NotifyMsg(msg []byte) {
	parsedMsg, err := DecodeMessage(msg)
	if err != nil {
		d.log.Error("failed to decode gossip message", zap.Error(err))
		return
	}

	var key string
	var device common.IgnoredDevice
	var removed bool

	switch parsedMsg.Type {
	case MessageTypeAddIgnored:
		key = parsedMsg.Device.Name
		device = parsedMsg.Device
	case MessageTypeRemoveIgnored:
		key = parsedMsg.Host
		device = common.IgnoredDevice{Name: parsedMsg.Host}
		removed = true
	default:
		d.log.Warn("unknown message type", zap.String("type", string(parsedMsg.Type)))
		return
	}

	// ApplyRemoteRecord does version comparison + local mutation atomically,
	// since memberlist may invoke delegate callbacks concurrently.
	if !common.ApplyRemoteRecord(key, device, removed, parsedMsg.Version, parsedMsg.NodeID) {
		d.log.Debug("ignoring stale gossip message", zap.String("host", key), zap.Int64("version", parsedMsg.Version))
		return
	}

	if removed {
		d.log.Debug("applied remove_ignored from gossip", zap.String("host", key))
	} else {
		d.log.Debug("applied add_ignored from gossip", zap.String("host", key))
	}
}

// GetBroadcasts implements memberlist.Delegate.GetBroadcasts
// This is called to get messages to broadcast
func (d *MessageDelegate) GetBroadcasts(overhead, limit int) [][]byte {
	broadcasts := d.manager.queue.GetBroadcasts(overhead, limit)
	return broadcasts
}

// LocalState implements memberlist.Delegate.LocalState
// This is called by memberlist's periodic push/pull anti-entropy exchange
// (and on join) to obtain this node's full view of ignored-host state,
// including tombstones for removed hosts, so that removals are not lost or
// resurrected during state reconciliation with peers.
func (d *MessageDelegate) LocalState(join bool) []byte {
	snapshot := common.SnapshotIgnoredRecords()
	data, err := json.Marshal(snapshot)
	if err != nil {
		d.log.Error("failed to marshal local state", zap.Error(err))
		return []byte("{}")
	}
	return data
}

// MergeRemoteState implements memberlist.Delegate.MergeRemoteState
// This is called to merge a remote peer's full state with local state.
// Conflicts are resolved via common.ApplyRemoteRecord/recordWins (Version,
// then NodeID as a tie-breaker).
func (d *MessageDelegate) MergeRemoteState(buf []byte, join bool) {
	remoteState := map[string]common.IgnoredRecord{}
	if len(buf) > 0 {
		if err := json.Unmarshal(buf, &remoteState); err != nil {
			d.log.Error("failed to unmarshal remote state", zap.Error(err))
			return
		}
	}

	for host, remoteRecord := range remoteState {
		// See NotifyMsg for why version comparison + local application must
		// be one atomic operation rather than two separately-locked steps.
		if !common.ApplyRemoteRecord(host, remoteRecord.Device, remoteRecord.Removed, remoteRecord.Version, remoteRecord.NodeID) {
			continue
		}

		if remoteRecord.Removed {
			d.log.Debug("merged ignored-host removal from remote state", zap.String("host", host))
		} else {
			d.log.Debug("merged ignored device from remote state", zap.String("host", host))
		}
	}
}

// EventDelegate implements memberlist.EventDelegate for membership changes
type EventDelegate struct {
	log *zap.Logger
}

// NotifyJoin implements memberlist.EventDelegate.NotifyJoin
func (e *EventDelegate) NotifyJoin(node *memberlist.Node) {
	e.log.Info("node joined cluster",
		zap.String("node_id", node.Name),
		zap.String("addr", node.Address()),
		zap.Uint16("port", node.Port))
}

// NotifyLeave implements memberlist.EventDelegate.NotifyLeave
func (e *EventDelegate) NotifyLeave(node *memberlist.Node) {
	e.log.Info("node left cluster",
		zap.String("node_id", node.Name),
		zap.String("addr", node.Address()))
}

// NotifyUpdate implements memberlist.EventDelegate.NotifyUpdate
func (e *EventDelegate) NotifyUpdate(node *memberlist.Node) {
	e.log.Debug("node updated",
		zap.String("node_id", node.Name),
		zap.String("addr", node.Address()))
}
