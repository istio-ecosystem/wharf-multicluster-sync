package agent

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
	ID string `json:"id" yaml:"ID"`

	GatewayIP   string `json:"gatewayIP" yaml:"GatewayIP"`
	GatewayPort uint16 `json:"gatewayPort" yaml:"GatewayPort"`

	AgentIP   string `json:"agentIP" yaml:"AgentIP"`
	AgentPort uint16 `json:"agentPort" yaml:"AgentPort"`

	ConnectionMode string `json:"connectionMode" yaml:"ConnectionMode"`

	WatchedPeers []ClusterConfig `json:"peers,omitempty" yaml:"WatchedPeers,omitempty"`
	TrustedPeers []string        `json:"trustedPeers,omitempty" yaml:"TrustedPeers,omitempty"`
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
