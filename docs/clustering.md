# Fishymetrics Clustering Guide

## Overview

Fishymetrics now supports distributed clustering using the Raft consensus protocol. This allows multiple instances to share state, including Vault tokens and ignored devices, while ensuring consistency across the cluster.

## Features

- **Raft Consensus**: Uses HashiCorp's Raft implementation for distributed consensus
- **Shared Vault Token**: Only the leader manages Vault token renewal, sharing it with followers
- **Leader-Only Writes**: Write operations (like removing ignored devices) can only be performed by the leader
- **Automatic Discovery**: Supports Kubernetes, DNS, and static peer discovery
- **Persistent State**: Cluster state is persisted to disk and survives restarts

## Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Node 1    │     │   Node 2    │     │   Node 3    │
│  (Leader)   │◄────┤  (Follower) │◄────┤  (Follower) │
└─────┬───────┘     └─────────────┘     └─────────────┘
      │                     ▲                     ▲
      │                     │                     │
      └─────────────────────┴─────────────────────┘
                    Raft Consensus
                    
      ┌──────────────────────────────────────────┐
      │           Shared State                    │
      │  - Vault Token                            │
      │  - Ignored Devices                        │
      │  - Custom Data                            │
      └──────────────────────────────────────────┘
```

## Configuration

### Environment Variables

```bash
# Enable clustering
CLUSTER_ENABLED=true

# Node identification
CLUSTER_NODE_ID=node-1  # Defaults to hostname

# Network configuration
CLUSTER_BIND_ADDR=0.0.0.0
CLUSTER_ADVERTISE_ADDR=10.0.0.1  # Defaults to POD_IP in K8s
CLUSTER_RAFT_PORT=7000
CLUSTER_DISCOVERY_PORT=7001

# Discovery configuration
CLUSTER_DISCOVERY_MODE=kubernetes  # Options: kubernetes, dns, static
CLUSTER_SERVICE_NAME=fishymetrics-headless
CLUSTER_NAMESPACE=default

# For static discovery
CLUSTER_STATIC_PEERS=10.0.0.2:7000,10.0.0.3:7000

# Data persistence
CLUSTER_DATA_DIR=/var/lib/fishymetrics/raft
```

### Command Line Flags

```bash
fishymetrics \
  --cluster.enabled=true \
  --cluster.node-id=node-1 \
  --cluster.bind-addr=0.0.0.0 \
  --cluster.advertise-addr=10.0.0.1 \
  --cluster.discovery-mode=kubernetes \
  --cluster.service-name=fishymetrics-headless
```

## Deployment

### Kubernetes StatefulSet

Deploy as a StatefulSet for stable network identities and persistent storage:

```bash
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/statefulset.yaml
```

The StatefulSet configuration includes:
- 3 replicas for high availability
- Persistent volume claims for Raft data
- Headless service for peer discovery
- Health and readiness probes

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
      - CLUSTER_STATIC_PEERS=fishymetrics-2:7000,fishymetrics-3:7000
    volumes:
      - raft-data-1:/var/lib/fishymetrics/raft
    ports:
      - "10023:10023"
      - "7000:7000"
      - "7001:7001"

  fishymetrics-2:
    image: fishymetrics:latest
    environment:
      - CLUSTER_ENABLED=true
      - CLUSTER_NODE_ID=node-2
      - CLUSTER_DISCOVERY_MODE=static
      - CLUSTER_STATIC_PEERS=fishymetrics-1:7000,fishymetrics-3:7000
    volumes:
      - raft-data-2:/var/lib/fishymetrics/raft

  fishymetrics-3:
    image: fishymetrics:latest
    environment:
      - CLUSTER_ENABLED=true
      - CLUSTER_NODE_ID=node-3
      - CLUSTER_DISCOVERY_MODE=static
      - CLUSTER_STATIC_PEERS=fishymetrics-1:7000,fishymetrics-2:7000
    volumes:
      - raft-data-3:/var/lib/fishymetrics/raft

volumes:
  raft-data-1:
  raft-data-2:
  raft-data-3:
```

## API Endpoints

### Cluster Status

```bash
# Get cluster status
GET /cluster/status

Response:
{
  "is_leader": true,
  "leader": "node-1",
  "shared_state": {
    "ignored_devices_count": 5,
    "has_vault_token": true
  }
}
```

### Cluster Health

```bash
# Get cluster health
GET /cluster/health

Response:
{
  "status": "healthy",
  "is_leader": true
}
```

### Leader-Only Operations

Write operations will redirect to the leader if attempted on a follower:

```bash
# Remove ignored device (leader-only)
POST /ignored/remove
{
  "host": "device-1"
}

# If not leader, returns:
{
  "error": "This node is not the leader. Write operations must be performed on the leader.",
  "leader": "node-1"
}
```

## Discovery Modes

### Kubernetes Discovery

Uses Kubernetes headless service for automatic peer discovery:
- Requires a headless service (ClusterIP: None)
- Pods automatically discover peers via DNS
- Best for Kubernetes deployments

### DNS Discovery

Uses DNS A records for peer discovery:
- Requires DNS server returning multiple A records
- Useful for cloud deployments with service discovery

### Static Discovery

Manually specify peer addresses:
- Best for fixed infrastructure
- Requires updating configuration when nodes change

## Monitoring

### Prometheus Metrics

The cluster exposes additional metrics:

```
# Raft state (1=leader, 0=follower)
fishymetrics_cluster_is_leader 1

# Number of peers
fishymetrics_cluster_peer_count 2

# Shared state metrics
fishymetrics_cluster_ignored_devices_count 5
fishymetrics_cluster_has_vault_token 1
```

### Logs

Important log messages:
- `cluster manager started` - Cluster initialized
- `successfully joined cluster via peer` - Node joined existing cluster
- `vault token updated in shared state` - Token shared with cluster
- `ignored device added via consensus` - State change applied

## Troubleshooting

### Node Not Joining Cluster

1. Check network connectivity between nodes
2. Verify discovery configuration
3. Check firewall rules for ports 7000 and 7001
4. Review logs for connection errors

### Split Brain

If cluster experiences split brain:
1. Stop all nodes
2. Clear Raft data directories
3. Start nodes one at a time
4. First node will become leader
5. Others will join as followers

### Token Renewal Issues

If Vault token renewal fails:
1. Check leader node logs
2. Verify Vault connectivity
3. Ensure AppRole credentials are valid
4. Token will be re-acquired automatically

## Best Practices

1. **Odd Number of Nodes**: Deploy 3 or 5 nodes for optimal consensus
2. **Persistent Storage**: Always use persistent volumes in production
3. **Network Stability**: Ensure stable network between nodes
4. **Monitoring**: Set up alerts for leader changes and token expiration
5. **Backup**: Regularly backup Raft data directory

## Security Considerations

1. **Network Security**: Use TLS for Raft communication in production
2. **Access Control**: Restrict access to cluster management endpoints
3. **Vault Integration**: Use separate AppRole for each environment
4. **Data Encryption**: Enable encryption at rest for persistent volumes

## Migration from Standalone

To migrate from standalone to clustered mode:

1. Deploy first node with clustering enabled
2. It will become the leader
3. Import existing ignored devices if needed
4. Deploy additional nodes
5. Verify cluster formation
6. Update load balancer to point to all nodes