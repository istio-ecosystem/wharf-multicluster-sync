// (C) Copyright IBM Corp. 2018. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package agent

import ()

const (
	// ConnectionModeKey is the labels key within the RemoteServiceBinding that
	// holds the mode for handling the Istio configs (from below modes)
	ConnectionModeKey = "connection"

	// ConnectionModeLive will imply that the reconciler will be called
	// automatically whenever a change in an RemoteServiceBinding has been
	// determined
	ConnectionModeLive = "live"

	// ConnectionModePotential will imply that the reconciler will NOT be
	// called whenever a change in an RemoteServiceBinding has been determined.
	// Istio configs will be generated once the mode is switched to 'live'.
	ConnectionModePotential = "potential"
)

// ExposedServices is a struct that holds list of entries each holding the
// information about an exposed service. JSON format of this struct is being
// sent back from a remote cluster's agent in response to an exposition request.
type ExposedServices struct {
	Services []*ExposedService
}

// ExposedService holds description of an exposed service that is visible to
// remote clusters.
type ExposedService struct {
	Name      string
	Namespace string
	Port      uint32
}

// ClusterConfig holds all the configuration information about the local
// cluster as well as the peered remote clusters.
type ClusterConfig struct {
	ID string `yaml:"ID"`

	GatewayIP   string `yaml:"GatewayIP"`
	GatewayPort uint16 `yaml:"GatewayPort"`

	AgentIP   string `yaml:"AgentIP"`
	AgentPort uint16 `yaml:"AgentPort"`

	ConnectionMode string `yaml:"ConnectionMode"`

	WatchedPeers []ClusterConfig `yaml:"WatchedPeers,omitempty"`
	TrustedPeers []string        `yaml:"TrustedPeers,omitempty"`
}

// IP is implementing the model.ClusterInfo interface
func (cc ClusterConfig) IP(name string) string {
	if name == cc.ID {
		return cc.GatewayIP
	}
	for _, peer := range cc.WatchedPeers {
		if name == peer.ID {
			return peer.GatewayIP
		}
	}
	return "255.255.255.255" // dummy value for unknown clusters
}

// Port is implementing the model.ClusterInfo interface
func (cc ClusterConfig) Port(name string) uint32 {
	if name == cc.ID {
		return uint32(cc.GatewayPort)
	}
	for _, peer := range cc.WatchedPeers {
		if name == peer.ID {
			return uint32(peer.GatewayPort)
		}
	}
	return 8080 // dummy value for unknown clusters
}
