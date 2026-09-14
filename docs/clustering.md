# Fishymetrics Clustering Guide

## Overview

Fishymetrics supports distributed clustering using a gossip protocol (via
[HashiCorp memberlist](https://github.com/hashicorp/memberlist)). This allows
multiple instances behind a load-balanced Kubernetes Service to share the
"ignored devices" list (hosts that failed BMC credential checks and are
temporarily skipped during scrapes) without requiring a leader/quorum, and
without any pod being a special case.

## Features

- **Gossip Protocol (SWIM)**: Uses HashiCorp's memberlist implementation for decentralized, eventually-consistent membership and message dissemination
- **No Leader**: Every node can accept both reads and writes; there is no leader election and no write bottleneck
- **Eventual Consistency**: Changes to the ignored-devices list propagate to all nodes within roughly a second in-cluster; this is acceptable since the ignored list is an operational/best-effort cache, not safety-critical state
- **Independent Vault Tokens**: Each pod manages its own Vault AppRole login/renewal; clustering does not coordinate Vault credentials
- **Automatic Discovery**: Supports Kubernetes (headless service DNS), plain DNS, and static peer discovery
- **No Persistent State**: Cluster membership and ignored-device state live in memory only; nothing is written to disk, and nothing needs to survive a pod restart (a restarted/rescheduled pod simply rejoins and receives a full state sync from its peers)
- **Kubernetes-friendly**: Nodes freely join and leave the mesh as pods scale up/down (e.g. via HPA) or roll during a deployment, with no manual membership reconfiguration required
- **Tombstoned, Versioned State**: Every add/remove is stamped with a monotonically increasing logical clock value and merged using last-write-wins conflict resolution, so periodic anti-entropy state exchanges can never resurrect a host that was already removed elsewhere (see [Conflict Resolution](#conflict-resolution-tombstones--versioning) below)

## Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Node 1    │◄───►│   Node 2    │◄───►│   Node 3    │
└─────────────┘     └─────────────┘     └─────────────┘
       ▲                                        ▲
       └────────────────────────────────────────┘
                  Gossip (SWIM protocol)
                  no leader, no quorum

Any node may:
  - service a scrape and locally detect a bad credential
  - broadcast an add/remove change to all peers
  - service a manual /ignored/add or /ignored/remove request
```

Each pod's Kubernetes Service traffic is load-balanced randomly across all
pods (the default `ClusterIP` Service behavior), and every pod is now equally
capable of handling any request.

## Conflict Resolution: Tombstones & Versioning

Gossip state propagates through memberlist in two independent ways, and both
have to agree on which of two conflicting updates "wins":

1. **`NotifyMsg` broadcast** — when a node calls `BroadcastAdd`/`BroadcastRemove`,
   the change is queued and disseminated to peers via memberlist's
   `TransmitLimitedQueue`. This is fast (sub-second) but is a best-effort,
   fire-and-forget UDP-based mechanism — a message can be delayed, delivered
   out of order, or occasionally dropped.
2. **`LocalState` / `MergeRemoteState` anti-entropy sync** — memberlist
   periodically performs a full-state push/pull exchange between random
   node pairs, independent of the broadcast queue, to repair any state that
   didn't make it through via broadcast (e.g. due to a dropped UDP packet,
   or a node that was briefly partitioned or joined late).

The anti-entropy sync must be able to resolve conflicting views of the same
host without simply favoring "presence wins": if `MergeRemoteState` only knew
how to *add* a host whenever a peer's snapshot contained it, a peer whose
snapshot hadn't yet caught up with a recent removal would resurrect that
host on every node it synced with. To prevent this, ignored-host state is
implemented as a small last-write-wins CRDT (conflict-free replicated data
type):

- Every `Message` (`gossip/messages.go`) carries a `Version int64` field —
  a logical clock derived from `time.Now().UnixNano()`, with a monotonic
  guard in `Manager.nextVersion()` so two rapid local calls (or a backwards
  system clock) can never produce a non-increasing value.
- `Manager` (`gossip/manager.go`) keeps an in-memory `map[string]versionedRecord`
  keyed by host, where each record stores the device data, a `Removed`
  tombstone flag, and the `Version` at which that state was last set. A
  remove is stored as a tombstone record rather than simply deleting the
  map entry — this is what allows a later, older-versioned "add" to be
  correctly rejected instead of resurrecting the host.
- Both `NotifyMsg` (broadcast path) and `MergeRemoteState` (anti-entropy
  path) funnel every incoming add/remove through a single method,
  `Manager.applyIfNewer(host, record)`, which only accepts the incoming
  record if `record.Version` is strictly greater than what's currently
  tracked for that host. Anything with an equal or older version is
  discarded as stale.
- `LocalState` serializes the full versioned/tombstoned state map (not just
  the currently-ignored hosts), so a peer that receives it during
  anti-entropy sync has enough information to correctly resolve removals,
  not just additions.

The net effect: no matter which of the two propagation paths a peer's view
of a host arrives through, and no matter what order updates are received
in, all nodes converge on the same state — the one with the highest
version — and an intentional removal can never be undone by a stale
snapshot. See `gossip/manager_test.go` (`TestNotifyMsgIgnoresStaleMessages`,
`TestMergeRemoteStateDoesNotResurrectRemovedHost`) for regression tests
covering this behavior directly.

## Configuration

### Environment Variables

```bash
# Enable clustering
CLUSTER_ENABLED=true

# Node identification
CLUSTER_NODE_ID=node-1  # Defaults to hostname; in Helm this is set to the pod name

# Network configuration
CLUSTER_BIND_ADDR=0.0.0.0
CLUSTER_ADVERTISE_ADDR=10.0.0.1  # Defaults to POD_IP in K8s
CLUSTER_GOSSIP_PORT=7946         # Single port used for both TCP and UDP gossip traffic

# Discovery configuration
CLUSTER_DISCOVERY_MODE=kubernetes  # Options: kubernetes, dns, static
CLUSTER_SERVICE_NAME=fishymetrics-headless
CLUSTER_NAMESPACE=default

# For static discovery
CLUSTER_STATIC_PEERS=10.0.0.2:7946,10.0.0.3:7946
```

### Command Line Flags

```bash
fishymetrics \
  --cluster.enabled \
  --cluster.node-id=node-1 \
  --cluster.bind-addr=0.0.0.0 \
  --cluster.advertise-addr=10.0.0.1 \
  --cluster.gossip-port=7946 \
  --cluster.discovery-mode=kubernetes \
  --cluster.service-name=fishymetrics-headless
```

## Deployment

### Kubernetes (Helm)

Clustering is fully wired into the Helm chart. Enable it via `values.yaml`:

```yaml
cluster:
  enabled: true
  gossipPort: 7946
  discoveryMode: "kubernetes"
  serviceName: "" # defaults to "<release-name>-fishymetrics-headless"
  namespace: ""   # defaults to the pod's own namespace
  staticPeers: ""
```

This automatically:
- Creates a headless Service (`clusterIP: None`) exposing the gossip port (TCP+UDP) for DNS-based peer discovery
- Adds the gossip container ports to the Deployment
- Injects `POD_IP` (via `status.podIP`), `CLUSTER_NODE_ID` (via `metadata.name`), and `CLUSTER_NAMESPACE` (via `metadata.namespace`) using the Kubernetes downward API
- Passes the relevant `--cluster.*` CLI flags

No StatefulSet or persistent volumes are required — clustering works with a
regular `Deployment` and scales freely with `horizontalPodAutoscaler.enabled: true`.

### Docker Compose

For local testing with Docker Compose:

```yaml
version: '3.8'

services:
  fishymetrics-1:
    image: fishymetrics:latest
    environment:
      - CLUSTER_ENABLED=true
      - CLUSTER_NODE_ID=node-1
      - CLUSTER_DISCOVERY_MODE=static
      - CLUSTER_STATIC_PEERS=fishymetrics-2:7946,fishymetrics-3:7946
    ports:
      - "10023:10023"
      - "7946:7946"

  fishymetrics-2:
    image: fishymetrics:latest
    environment:
      - CLUSTER_ENABLED=true
      - CLUSTER_NODE_ID=node-2
      - CLUSTER_DISCOVERY_MODE=static
      - CLUSTER_STATIC_PEERS=fishymetrics-1:7946,fishymetrics-3:7946

  fishymetrics-3:
    image: fishymetrics:latest
    environment:
      - CLUSTER_ENABLED=true
      - CLUSTER_NODE_ID=node-3
      - CLUSTER_DISCOVERY_MODE=static
      - CLUSTER_STATIC_PEERS=fishymetrics-1:7946,fishymetrics-2:7946
```

## API Endpoints

### Cluster Status

```bash
GET /cluster/status

Response:
{
  "local_node": "node-1",
  "member_count": 3,
  "members": [
    {"name": "node-1", "addr": "10.0.0.1:7946"},
    {"name": "node-2", "addr": "10.0.0.2:7946"},
    {"name": "node-3", "addr": "10.0.0.3:7946"}
  ],
  "ignored_devices_count": 5
}
```

### Cluster Health

```bash
GET /cluster/health

Response:
{
  "status": "healthy",
  "local_node": "node-1",
  "member_count": 3
}
```

### Ignored-Host Operations (any node)

Writes are **not** restricted to a leader — any node accepts and services
these requests, applying the change locally and broadcasting it to the rest
of the cluster:

```bash
# Remove ignored device (any node)
POST /ignored/remove
{
  "host": "device-1"
}

# Add ignored device (any node, cluster mode only)
POST /ignored/add
{
  "Name": "device-1",
  "Endpoint": "https://device-1/redfish/v1/Chassis/",
  "Model": "...",
  "CredentialProfile": "default"
}
```

Additionally, when a scrape detects invalid BMC credentials, the host is
automatically added to the ignored list on the node that performed the
scrape **and broadcast to the rest of the cluster**.

## Discovery Modes

### Kubernetes Discovery

Uses the headless Service for automatic peer discovery via DNS (`A`/`AAAA`
records resolve to individual pod IPs):
- Requires a headless service (`clusterIP: None`) — provided automatically by the Helm chart when `cluster.enabled: true`
- Pods discover peers via DNS, then join the gossip mesh (memberlist's own protocol handles the rest of membership propagation)
- Best for Kubernetes deployments

### DNS Discovery

Uses DNS `A` records for peer discovery:
- Requires a DNS server returning multiple `A` records for the configured name
- Useful for cloud deployments with external service discovery

### Static Discovery

Manually specify peer addresses:
- Best for fixed infrastructure
- Requires updating configuration when nodes change

## Monitoring

### Logs

Important log messages:
- `gossip cluster manager started` — cluster initialized
- `joined cluster` — node joined an existing mesh
- `node joined cluster` / `node left cluster` — membership change detected
- `added host ... to ignored list` — local scrape detected a bad credential

## Troubleshooting

### Node Not Joining Cluster

1. Check network connectivity between pods (both TCP and UDP on the gossip port must be reachable)
2. Verify the headless service exists and resolves to pod IPs (`kubectl get endpoints <service>-headless`)
3. Check firewall/NetworkPolicy rules for the gossip port (default `7946`)
4. Review logs for peer discovery/join errors

### Divergent State

Because this is an eventually-consistent system, a brief window of staleness
across nodes is expected and acceptable (typically resolves within ~1s in a
healthy cluster). Conflicting updates are resolved automatically via the
versioned tombstone model described in
[Conflict Resolution](#conflict-resolution-tombstones--versioning) — the
update with the highest version always wins, and a removal can never be
undone by a peer's stale snapshot. If nodes remain divergent for longer:
1. Confirm all nodes report each other via `GET /cluster/status`
2. Check for a network partition between nodes
3. Restart the affected pod — it will rejoin and receive a full state sync (`LocalState`/`MergeRemoteState`) from any existing peer

## Best Practices

1. **No minimum node count**: there's no quorum requirement — a single node cluster is a valid (if temporary) state
2. **Network Stability**: ensure both TCP and UDP are permitted between pods on the gossip port
3. **Monitoring**: alert on `member_count` from `/cluster/status` dropping below the expected replica count
4. **HPA-friendly**: safe to scale replicas up/down freely; new pods join automatically, terminated pods are detected and removed from membership via SWIM failure detection

## Security Considerations

1. **Network Security**: gossip traffic is unencrypted and unauthenticated by default; restrict the gossip port to intra-cluster traffic via NetworkPolicy. memberlist also supports a shared-key encryption mode (`SecretKey` on the memberlist config) if needed in the future.
2. **Access Control**: restrict access to `/cluster/status`, `/cluster/health`, and `/ignored/*` endpoints at the ingress/network level
