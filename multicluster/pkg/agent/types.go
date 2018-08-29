package agent

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
	Port      uint16
}

// PeerAgent holds relevant information about a peered cluster
type PeerAgent struct {
	ID      string
	Address string
	Port    uint16
}

type DebugClusterInfo struct {
	IPs   map[string]string
	Ports map[string]uint32
}

func (ci DebugClusterInfo) Ip(name string) string {
	out, ok := ci.IPs[name]
	if ok {
		return out
	}
	return "255.255.255.255" // dummy value for unknown clusters
}

func (ci DebugClusterInfo) Port(name string) uint32 {
	out, ok := ci.Ports[name]
	if ok {
		return out
	}
	return 8080 // dummy value for unknown clusters
}
