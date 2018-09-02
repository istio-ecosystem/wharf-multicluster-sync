package agent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.ibm.com/istio-research/multicluster-roadmap/multicluster/pkg/reconcile"

	"github.ibm.com/istio-research/multicluster-roadmap/multicluster/pkg/config/kube/crd"

	"github.ibm.com/istio-research/multicluster-roadmap/api/multicluster/v1alpha1"
	mcmodel "github.ibm.com/istio-research/multicluster-roadmap/multicluster/pkg/model"
	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pkg/log"
)

const (
	pollInterval = 5 * time.Second
)

// Client is an agent client meant to connect to an agent server on a peered
// remote cluster and poll for updates on time intervals. Fetched configuration
// will be transformed into local RemoteServiceBinding resources.
type Client struct {
	config       *ClusterConfig
	peer         *ClusterConfig
	crdClient    *crd.Client
	pollInterval time.Duration

	store      mcmodel.MCConfigStore
	istioStore model.ConfigStore
}

// NewClient will create a new agent client that connects to a peered server on
// the specified address:port and fetch current exposition policies. The client
// will start polling only when the Run() function is called.
func NewClient(config *ClusterConfig, peer *ClusterConfig, client *crd.Client, store *mcmodel.MCConfigStore) (*Client, error) {
	c := &Client{
		config:       config,
		peer:         peer,
		crdClient:    client,
		pollInterval: pollInterval,
		store:        *store,
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

	// If returned query response is 0 exposed services it can either be that
	// all exposed services were removed or there weren't any in the first
	// place.
	if len(exposed.Services) == 0 {
		// If all previously exposed services were removed we cleanup all
		// related Istio configs and the relevant RemoteServiceBinding
		if rsb := c.remoteServiceBinding(); rsb != nil {
			log.Debugf("Peer [%s] removed all exposed services", c.peer.ID)
			// First use the API server to create the new binding
			err := c.crdClient.Delete(rsb.Type, rsb.Name, rsb.Namespace)
			if err != nil {
				log.Errora(err)
			}
			// Peer removed all exposed services therefore no need for the RSB
			c.store.Delete(rsb.Type, rsb.Name, rsb.Namespace)
			if err != nil {
				log.Errora(err)
			}
			log.Debugf("RemoteServiceVinfing deleted for cluser [%s] deleted", c.peer.ID)

			// Use the reconcile to generate the inferred Istio configs for the new binding
			deleted, err := reconcile.DeleteMulticlusterConfig(c.store, *rsb, c.config)
			if err != nil {
				log.Errora(err)
				return
			}
			StoreIstioConfigs(c.istioStore, nil, nil, deleted)
		}
		return
	}

	if !c.needsUpdate(exposed) {
		// Nothing changed on peered cluster since last check
		return
	}

	// TODO handle updates
	binding := c.exposedServicesToBinding(exposed)
	c.createRemoteServiceBinding(binding)
}

func (c *Client) createRemoteServiceBinding(binding *model.Config) {
	// First use the API server to create the new binding
	_, err := c.crdClient.Create(*binding)
	if err != nil {
		log.Errora(err)
		return
	}
	log.Debug("RemoteServiceBinding created for the exposed remote service(s)")

	// Add it to the config store
	c.store.Create(*binding)

	// Use the reconcile to generate the inferred Istio configs for the new binding
	added, modified, err := reconcile.AddMulticlusterConfig(c.store, *binding, c.config)
	if err != nil {
		log.Errora(err)
		return
	}
	StoreIstioConfigs(c.istioStore, added, modified, nil)
}

// Function will call the peer of this client and fetch the current state of
// exposed services.
func (c *Client) callPeer() (*ExposedServices, error) {
	url := fmt.Sprintf("http://%s:%d/exposed/%s", c.peer.AgentIP, c.peer.AgentPort, c.config.ID)
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

// Go through the provided list of exposed services and determine whether there
// are any updates/changes from the current list. Organize the changes and
// return those.
func (c *Client) needsUpdate(exposed *ExposedServices) bool {
	current := c.store.RemoteServiceBindings()
	for _, rsb := range current {
		spec, _ := rsb.Spec.(*v1alpha1.RemoteServiceBinding)
		for _, remote := range spec.Remote {
			if remote.Cluster == c.peer.ID { // found it
				if len(remote.Services) != len(exposed.Services) {
					return true
				}
				// TODO go through both lists and see if there are any differences
				return false
			}
		}
	}
	return true
}

func (c *Client) exposedServicesToBinding(exposed *ExposedServices) *model.Config {
	services := make([]*v1alpha1.RemoteServiceBinding_RemoteCluster_RemoteService, len(exposed.Services))
	ns := ""
	for i, service := range exposed.Services {
		services[i] = &v1alpha1.RemoteServiceBinding_RemoteCluster_RemoteService{
			Name:      service.Name,
			Alias:     service.Name,
			Namespace: service.Namespace,
		}
		ns = service.Namespace
	}
	name := strings.ToLower(c.peer.ID) + "-services"
	return &model.Config{
		ConfigMeta: model.ConfigMeta{
			Type:      mcmodel.RemoteServiceBinding.Type,
			Group:     mcmodel.RemoteServiceBinding.Group,
			Version:   mcmodel.RemoteServiceBinding.Version,
			Name:      name,
			Namespace: ns,
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

// Go through the RemoteServiceBindings in the store and find the one
// relevant for the peered cluster.
// TODO handle more than one RemoteServiceBinding for the cluster
func (c *Client) remoteServiceBinding() *model.Config {
	for _, rsb := range c.store.RemoteServiceBindings() {
		spec, _ := rsb.Spec.(*v1alpha1.RemoteServiceBinding)
		for _, remote := range spec.Remote {
			if remote.Cluster == c.peer.ID { // found it
				return &rsb
			}
		}
	}
	return nil
}
