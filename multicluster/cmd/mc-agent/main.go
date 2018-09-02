package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.ibm.com/istio-research/multicluster-roadmap/multicluster/pkg/agent"
	"github.ibm.com/istio-research/multicluster-roadmap/multicluster/pkg/config/kube/crd"
	mcmodel "github.ibm.com/istio-research/multicluster-roadmap/multicluster/pkg/model"
	"github.ibm.com/istio-research/multicluster-roadmap/multicluster/pkg/reconcile"

	"istio.io/istio/pilot/pkg/config/memory"
	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pilot/pkg/serviceregistry/kube"
	"istio.io/istio/pkg/log"

	// Importing the API messages so that when resource events are fired the
	// resource will be parsed into a message object
	_ "github.ibm.com/istio-research/multicluster-roadmap/api/multicluster/v1alpha1"
)

const (
	resyncPeriod = 1 * time.Second
)

var (
	namespace  string
	kubeconfig string
	context    string
	configJSON string

	store         mcmodel.MCConfigStore
	istioStore    model.ConfigStore
	clusterConfig agent.ClusterConfig
)

func main() {
	flag.Parse()

	if configJSON == "" {
		log.Error("Cluster configuration JSON file must be provided with the -configJson flag")
		return
	}

	// Load the cluster config from the provided json file
	clusterConfig, err := loadConfig(configJSON)
	if err != nil {
		log.Errora(err)
		return
	}

	// Setting up an in-memory config store for MultiCluster config types
	store = mcmodel.MakeMCStore(memory.Make(mcmodel.MultiClusterConfigTypes))

	// Setting up an in-memory config store for Istio config types
	istioStore = model.MakeIstioStore(memory.Make(model.IstioConfigTypes))

	// Set up a Kubernetes API client for the Multi-Cluster configs
	desc := model.ConfigDescriptor{mcmodel.ServiceExpositionPolicy, mcmodel.RemoteServiceBinding}
	cl, err := crd.NewClient(kubeconfig, context, desc, "")
	if err != nil {
		log.Errora(err)
		return
	}

	// Register the Multi-Cluster CRDs if they aren't already registered
	err = cl.RegisterResources()
	if err != nil {
		log.Errora(err)
		return
	}

	// Setting up a controller for the configured namespace to periodically watch for changes
	ctl := crd.NewController(cl, kube.ControllerOptions{WatchedNamespace: namespace, ResyncPeriod: resyncPeriod})

	// Register model configs event handler that will update the config store accordingly
	ctl.RegisterEventHandler(mcmodel.ServiceExpositionPolicy.Type, func(config model.Config, ev model.Event) {
		switch ev {
		case model.EventAdd:
			log.Debugf("ServiceExpositionPolicy resource was added. Name: %s.%s", config.Namespace, config.Name)
			log.Debug("Adding it to the config store..")
			store.Create(config)

			log.Debug("Generate and add reconciled Istio configs..")
			added, modified, err := reconcile.AddMulticlusterConfig(istioStore, config, clusterConfig)
			if err != nil {
				log.Errora(err)
				return
			}
			agent.StoreIstioConfigs(istioStore, added, modified, nil)
		case model.EventDelete:
			log.Debugf("ServiceExpositionPolicy resource was deleted. Name: %s.%s", config.Namespace, config.Name)
			log.Debug("Deleting it from the config store..")
			store.Delete(config.Type, config.Name, config.Namespace)

			log.Debug("Delete the relevant Istio configs..")
			deleted, err := reconcile.DeleteMulticlusterConfig(istioStore, config, clusterConfig)
			if err != nil {
				log.Errora(err)
				return
			}
			agent.StoreIstioConfigs(istioStore, nil, nil, deleted)
		case model.EventUpdate:
			log.Debugf("ServiceExpositionPolicy resource was updated. Name: %s.%s", config.Namespace, config.Name)
			log.Debug("Updating it in the config store..")
			store.Update(config)

			log.Debug("Update the relevant Istio configs..")
			updated, err := reconcile.ModifyMulticlusterConfig(istioStore, config, clusterConfig)
			if err != nil {
				log.Errora(err)
				return
			}
			agent.StoreIstioConfigs(istioStore, nil, updated, nil)
		}
		log.Debugf("Config store now has %d ServiceExpositionPolicy entries", len(store.ServiceExpositionPolicies()))
	})

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	stopCh := make(chan struct{})

	log.Debug("Starting controller..")
	go ctl.Run(stopCh)

	log.Debugf("Starting agent listener on port %d..", clusterConfig.AgentPort)
	server, err := agent.NewServer(clusterConfig.AgentIP, clusterConfig.AgentPort, store)
	go server.Run()

	log.Debugf("Starting agent clients. Number of peers: %d", len(clusterConfig.Peers))
	clients := []*agent.Client{}
	for _, peer := range clusterConfig.Peers {
		client, err := agent.NewClient(clusterConfig, &peer, cl, &store)
		if err != nil {
			log.Errorf("Failed to create an agent client to peer: %s", peer.ID)
			continue
		}
		go client.Run(stopCh)
		clients = append(clients, client)
	}

	<-shutdown
	log.Debug("Shutting down the Multi-Cluster agent")

	close(stopCh)
	server.Close()

	_ = log.Sync()
}

// loadConfig will load the cluster configuration from the provided JSON file
func loadConfig(file string) (*agent.ClusterConfig, error) {
	jsonFile, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()

	var config agent.ClusterConfig
	bytes, _ := ioutil.ReadAll(jsonFile)
	err = json.Unmarshal(bytes, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func init() {
	// set up Istio logger
	o := log.DefaultOptions()
	o.SetOutputLevel(log.DefaultScopeName, log.DebugLevel)
	if err := log.Configure(o); err != nil {
		fmt.Printf("Failed to configure logger: %v", err)
		return
	}

	flag.StringVar(&configJSON, "configJson", "", "Config JSON file to use for the agent configuration")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&context, "context", "", "Kubeconfig context to be used. Only required if out-of-cluster.")
	flag.StringVar(&namespace, "namespace", "", "Namespace to watch. Default (or empty string) is all namespaces.")
}
