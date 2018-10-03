package agent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/istio-ecosystem/wharf-multicluster-sync/api/multicluster/v1alpha1"
	mcmodel "github.com/istio-ecosystem/wharf-multicluster-sync/multicluster/pkg/model"
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
	pollInterval time.Duration

	store      mcmodel.MCConfigStore
	istioStore model.ConfigStore
}

// NewClient will create a new agent client that connects to a peered server on
// the specified address:port and fetch current exposition policies. The client
// will start polling only when the Run() function is called.
func NewClient(config *ClusterConfig, peer *ClusterConfig, store *mcmodel.MCConfigStore, istioStore model.ConfigStore) (*Client, error) {
	c := &Client{
		config:       config,
		peer:         peer,
		pollInterval: pollInterval,
		store:        *store,
		istioStore:   istioStore,
	}
	return c, nil
}

// Run will start...
func (c *Client) Run(cfgCh chan ClusterConfig, stopCh chan struct{}) {
	log.Debugf("Configuration for peer [%s]:\nConnection mode: %s\nAgent: %s:%d\nGateway: %s:%d",
		c.peer.ID, c.peer.ConnectionMode, c.peer.AgentIP, c.peer.AgentPort, c.peer.GatewayIP, c.peer.GatewayPort)
	go func() {
		// start polling
		tick := time.Tick(c.pollInterval)
		for {
			select {
			case cfg, ok := <-cfgCh:
				if !ok {
					c.close()
					return
				}
				c.configUpdated(&cfg)
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

	// Get the connection mode for the peer. Can either be live or potential.
	// In live mode the Istio Configs will be created and deleted.
	connMode := c.peer.ConnectionMode
	if connMode == "" {
		connMode = ConnectionModeLive
	}

	// If returned query response is 0 exposed services it can either be that
	// all exposed services were removed or there weren't any in the first
	// place.
	// if len(exposed.Services) == 0 {
	// 	// If all previously exposed services were removed we cleanup all
	// 	// related Istio configs and the relevant RemoteServiceBinding
	// 	if rsb := c.remoteServiceBinding(); rsb != nil {
	// 		log.Debugf("Peer [%s] removed all exposed services", c.peer.ID)

	// 		// Peer removed all exposed services therefore no need for the RSB
	// 		c.store.Delete(rsb.Type, rsb.Name, rsb.Namespace)
	// 		if err != nil {
	// 			log.Errora(err)
	// 		}
	// 		log.Debugf("RemoteServiceBinding for cluser [%s] deleted", c.peer.ID)

	// 		// Delete Istio configs if in live mode
	// 		if connMode == ConnectionModeLive {
	// 			c.reconcile(nil, rsb)
	// 		}
	// 	}
	// 	return
	// }

	// TODO: currently just checking if something has changed by comparing the number of services.
	// This needs to be revisited to compare existing and incoming services and figure what are
	// the added, updated and deleted services.
	if !c.needsUpdate(exposed) {
		// Nothing changed on peered cluster since last check
		return
	}

	// TODO: Below is an unefficient implementation to handle update which first delete the RSB
	// and its Istio configs and then create new ones with the updated info.

	oldRsb := c.remoteServiceBinding()
	if oldRsb != nil {
		c.store.Delete(mcmodel.RemoteServiceBinding.Type, oldRsb.Name, oldRsb.Namespace)
		log.Debug("Old RemoteServiceBinding deleted for the exposed remote service(s)")
	}

	newRsb := c.createRemoteServiceBinding(exposed, connMode)
	if newRsb != nil {
		// Add it to the config store
		c.store.Create(*newRsb)
		log.Debug("RemoteServiceBinding created for the exposed remote service(s)")
	}
}

// Create a RemoteServiceBinding object for the exposed services
func (c *Client) createRemoteServiceBinding(exposed *ExposedServices, connectionMode string) *model.Config {
	if exposed == nil || len(exposed.Services) == 0 {
		return nil
	}
	services := make([]*v1alpha1.RemoteServiceBinding_RemoteCluster_RemoteService, len(exposed.Services))
	ns := ""
	for i, service := range exposed.Services {
		services[i] = &v1alpha1.RemoteServiceBinding_RemoteCluster_RemoteService{
			Name:      service.Name,
			Alias:     service.Name,
			Namespace: service.Namespace,
			Port:      service.Port,
		}
		ns = service.Namespace
	}
	name := strings.ToLower(c.peer.ID) + "-services"
	return &model.Config{
		ConfigMeta: model.ConfigMeta{
			Type:      mcmodel.RemoteServiceBinding.Type,
			Group:     mcmodel.RemoteServiceBinding.Group + model.IstioAPIGroupDomain,
			Version:   mcmodel.RemoteServiceBinding.Version,
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				ConnectionModeKey: connectionMode,
			},
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
		return nil, fmt.Errorf("response status code is not OK")
	}

	exposed := &ExposedServices{}
	err = json.NewDecoder(resp.Body).Decode(exposed)
	if err != nil {
		return nil, fmt.Errorf("failed to decode the response JSON: %v", err)
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

// When new agent config arrives the function will update agent and the peer
// configs specific to this client.
func (c *Client) configUpdated(newConfig *ClusterConfig) {
	c.config = newConfig
	for _, newPeer := range newConfig.WatchedPeers {
		if newPeer.ID == c.peer.ID {
			c.peer = &newPeer
		}
	}
}
