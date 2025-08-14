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
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
)

type DiscoveryMode string

const (
	DiscoveryModeKubernetes DiscoveryMode = "kubernetes"
	DiscoveryModeStatic     DiscoveryMode = "static"
	DiscoveryModeDNS        DiscoveryMode = "dns"
)

type DiscoveryConfig struct {
	Mode            DiscoveryMode
	ServiceName     string   // For Kubernetes headless service or DNS
	Namespace       string   // For Kubernetes
	StaticPeers     []string // For static mode
	RaftPort        int      // Port for Raft communication
	DiscoveryPort   int      // Port for discovery/join API
	Logger          *zap.Logger
}

type ClusterDiscovery struct {
	config *DiscoveryConfig
	log    *zap.Logger
}

func NewClusterDiscovery(config *DiscoveryConfig) *ClusterDiscovery {
	if config.Logger == nil {
		config.Logger = zap.L()
	}

	if config.RaftPort == 0 {
		config.RaftPort = 7000
	}

	if config.DiscoveryPort == 0 {
		config.DiscoveryPort = 7001
	}

	return &ClusterDiscovery{
		config: config,
		log:    config.Logger,
	}
}

func (d *ClusterDiscovery) DiscoverPeers(ctx context.Context) ([]string, error) {
	switch d.config.Mode {
	case DiscoveryModeKubernetes:
		return d.discoverKubernetesPeers(ctx)
	case DiscoveryModeDNS:
		return d.discoverDNSPeers(ctx)
	case DiscoveryModeStatic:
		return d.config.StaticPeers, nil
	default:
		return nil, fmt.Errorf("unknown discovery mode: %s", d.config.Mode)
	}
}

func (d *ClusterDiscovery) discoverKubernetesPeers(ctx context.Context) ([]string, error) {
	namespace := d.config.Namespace
	if namespace == "" {
		// Try to read namespace from Kubernetes service account
		data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err != nil {
			return nil, fmt.Errorf("failed to read namespace: %w", err)
		}
		namespace = strings.TrimSpace(string(data))
	}

	// Use headless service DNS for discovery
	serviceDNS := fmt.Sprintf("%s.%s.svc.cluster.local", d.config.ServiceName, namespace)
	return d.resolveDNS(serviceDNS)
}

func (d *ClusterDiscovery) discoverDNSPeers(ctx context.Context) ([]string, error) {
	return d.resolveDNS(d.config.ServiceName)
}

func (d *ClusterDiscovery) resolveDNS(hostname string) ([]string, error) {
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve %s: %w", hostname, err)
	}

	var peers []string
	for _, ip := range ips {
		// Skip IPv6 addresses for now
		if ip.To4() != nil {
			peer := fmt.Sprintf("%s:%d", ip.String(), d.config.RaftPort)
			peers = append(peers, peer)
		}
	}

	return peers, nil
}

func (d *ClusterDiscovery) JoinCluster(ctx context.Context, raftNode *RaftNode) error {
	peers, err := d.DiscoverPeers(ctx)
	if err != nil {
		return fmt.Errorf("failed to discover peers: %w", err)
	}

	if len(peers) == 0 {
		d.log.Info("no peers discovered, starting as single node")
		return nil
	}

	// Try to join an existing cluster
	for _, peer := range peers {
		peerAddr := strings.Replace(peer, fmt.Sprintf(":%d", d.config.RaftPort), 
			fmt.Sprintf(":%d", d.config.DiscoveryPort), 1)
		
		if err := d.tryJoinPeer(ctx, peerAddr, raftNode); err != nil {
			d.log.Warn("failed to join peer", zap.String("peer", peerAddr), zap.Error(err))
			continue
		}
		
		d.log.Info("successfully joined cluster via peer", zap.String("peer", peerAddr))
		return nil
	}

	d.log.Warn("failed to join any existing peers, may need to bootstrap")
	return nil
}

func (d *ClusterDiscovery) tryJoinPeer(ctx context.Context, peerAddr string, raftNode *RaftNode) error {
	joinRequest := map[string]string{
		"node_id": raftNode.nodeID,
		"addr":    raftNode.advertiseAddr,
	}

	data, err := json.Marshal(joinRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal join request: %w", err)
	}

	url := fmt.Sprintf("http://%s/cluster/join", peerAddr)
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send join request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("join request failed with status: %d", resp.StatusCode)
	}

	return nil
}

func (d *ClusterDiscovery) StartDiscoveryServer(raftNode *RaftNode) {
	mux := http.NewServeMux()

	mux.HandleFunc("/cluster/join", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var joinRequest map[string]string
		if err := json.NewDecoder(r.Body).Decode(&joinRequest); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		nodeID := joinRequest["node_id"]
		addr := joinRequest["addr"]

		if nodeID == "" || addr == "" {
			http.Error(w, "node_id and addr are required", http.StatusBadRequest)
			return
		}

		if err := raftNode.JoinCluster(nodeID, addr); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "joined"})
	})

	mux.HandleFunc("/cluster/status", func(w http.ResponseWriter, r *http.Request) {
		status := map[string]interface{}{
			"node_id":   raftNode.nodeID,
			"is_leader": raftNode.IsLeader(),
			"leader":    raftNode.GetLeader(),
			"stats":     raftNode.Stats(),
		}

		peers, err := raftNode.GetPeers()
		if err == nil {
			peerList := make([]map[string]string, 0, len(peers))
			for _, peer := range peers {
				peerList = append(peerList, map[string]string{
					"id":      string(peer.ID),
					"address": string(peer.Address),
				})
			}
			status["peers"] = peerList
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	})

	addr := fmt.Sprintf(":%d", d.config.DiscoveryPort)
	d.log.Info("starting discovery server", zap.String("addr", addr))
	
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			d.log.Error("discovery server failed", zap.Error(err))
		}
	}()
}