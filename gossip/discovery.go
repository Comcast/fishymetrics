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
	"fmt"
	"net"
	"os"
	"strings"

	"go.uber.org/zap"
)

type DiscoveryMode string

const (
	DiscoveryModeKubernetes DiscoveryMode = "kubernetes"
	DiscoveryModeDNS        DiscoveryMode = "dns"
	DiscoveryModeStatic     DiscoveryMode = "static"
)

type DiscoveryConfig struct {
	Mode        DiscoveryMode
	ServiceName string   // For Kubernetes headless service or DNS
	Namespace   string   // For Kubernetes
	StaticPeers []string // For static mode
	GossipPort  int      // Port for gossip communication
	Logger      *zap.Logger
}

type ClusterDiscovery struct {
	config *DiscoveryConfig
	log    *zap.Logger
}

func NewClusterDiscovery(config *DiscoveryConfig) *ClusterDiscovery {
	if config.Logger == nil {
		config.Logger = zap.L()
	}

	if config.GossipPort == 0 {
		config.GossipPort = 7946
	}

	return &ClusterDiscovery{
		config: config,
		log:    config.Logger,
	}
}

// DiscoverPeers returns a list of peer addresses in the format "ip:port"
func (d *ClusterDiscovery) DiscoverPeers() ([]string, error) {
	switch d.config.Mode {
	case DiscoveryModeKubernetes:
		return d.discoverKubernetesPeers()
	case DiscoveryModeDNS:
		return d.discoverDNSPeers()
	case DiscoveryModeStatic:
		return d.config.StaticPeers, nil
	default:
		return nil, fmt.Errorf("unknown discovery mode: %s", d.config.Mode)
	}
}

func (d *ClusterDiscovery) discoverKubernetesPeers() ([]string, error) {
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

func (d *ClusterDiscovery) discoverDNSPeers() ([]string, error) {
	return d.resolveDNS(d.config.ServiceName)
}

func (d *ClusterDiscovery) resolveDNS(hostname string) ([]string, error) {
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve %s: %w", hostname, err)
	}

	var peers []string
	for _, ip := range ips {
		// Include both IPv4 and IPv6
		peer := fmt.Sprintf("%s:%d", ip.String(), d.config.GossipPort)
		peers = append(peers, peer)
	}

	return peers, nil
}
