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

type PeerAgent struct {
	ID      string
	Address string
	Port    uint16
}
