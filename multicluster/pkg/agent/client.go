package agent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.ibm.com/istio-research/multicluster-roadmap/multicluster/pkg/config/kube/crd"

	"github.ibm.com/istio-research/multicluster-roadmap/api/multicluster/v1alpha1"
	"github.ibm.com/istio-research/multicluster-roadmap/multicluster/pkg/model"
	istiomodel "istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pkg/log"
)

const (
	pollInterval = 5 * time.Second
)

// Client is an agent client meant to connect to an agent server on a peered
// remote cluster and poll for updates on time intervals. Fetched configuration
// will be transformed into local RemoteServiceBinding resources.
type Client struct {
	crdClient    *crd.Client
	clusterID    string
	peer         PeerAgent
	pollInterval time.Duration
}

// NewClient will create a new agent client that connects to a peered server on
// the specified address:port and fetch current exposition policies. The client
// will start polling only when the Run() function is called.
func NewClient(clusterID string, peerAgent PeerAgent, client *crd.Client) (*Client, error) {
	c := &Client{
		clusterID:    clusterID,
		peer:         peerAgent,
		crdClient:    client,
		pollInterval: pollInterval,
	}
	return c, nil
}

// Run will start...
func (c *Client) Run(stopCh chan struct{}) {
	go func() {
		// start polling
		tick := time.Tick(c.pollInterval)
		for {
			select {
			case <-stopCh:
				c.close()
				return
			case <-tick:
				c.update()
			}
		}
	}()
}

// cleans up resources used by the server.
func (c *Client) close() {
	log.Debug("Agent client stopped")
}

func (c *Client) update() {
	exposed, err := c.callPeer()
	if err != nil {
		log.Debugf("Peer agent [%s] is not accessible. %v", c.peer.ID, err)
		return
	}
	log.Debugf("Number of exposed services on cluster [%s]: %d", c.peer.ID, len(exposed.Services))

	if !c.needsUpdate(exposed) {
		log.Debug("Nothing changed on peered cluster")
		return
	}

	binding := c.exposedServicesToBinding(exposed)
	_, err = c.crdClient.Create(*binding)
	if err != nil {
		log.Errora(err)
		return
	}
	log.Debug("RemoteServiceBinding created for the exposed remote service(s)")

}

func (c *Client) callPeer() (*ExposedServices, error) {
	url := fmt.Sprintf("http://%s:%d/exposed/%s", c.peer.Address, c.peer.Port, c.clusterID)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Response status code is not OK")
	}

	exposed := &ExposedServices{}
	err = json.NewDecoder(resp.Body).Decode(exposed)
	if err != nil {
		return nil, fmt.Errorf("Failed to decode the response JSON: %v", err)
	}
	return exposed, nil
}

func (c *Client) needsUpdate(exposed *ExposedServices) bool {
	if len(exposed.Services) == 0 {
		return false
	}
	return true
}

func (c *Client) exposedServicesToBinding(exposed *ExposedServices) *istiomodel.Config {
	services := make([]*v1alpha1.RemoteServiceBinding_RemoteCluster_RemoteService, len(exposed.Services))
	for i, service := range exposed.Services {
		services[i] = &v1alpha1.RemoteServiceBinding_RemoteCluster_RemoteService{
			Name:  service.Name,
			Alias: service.Name,
		}
	}
	name := c.peer.ID + "-services"
	return &istiomodel.Config{
		ConfigMeta: istiomodel.ConfigMeta{
			Type:      model.RemoteServiceBinding.Type,
			Group:     model.RemoteServiceBinding.Group,
			Version:   model.RemoteServiceBinding.Version,
			Name:      name,
			Namespace: "",
		},
		Spec: &v1alpha1.RemoteServiceBinding{
			Remote: []*v1alpha1.RemoteServiceBinding_RemoteCluster{
				&v1alpha1.RemoteServiceBinding_RemoteCluster{
					Cluster:  c.peer.ID,
					Services: services,
				},
			},
		},
	}
}
