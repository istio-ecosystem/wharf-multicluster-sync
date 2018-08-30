package main

import (
	"flag"
	"fmt"
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
	id         string
	port       int

	peers       []agent.PeerAgent
	hasDemoPeer bool

	store       mcmodel.MCConfigStore
	clusterInfo agent.DebugClusterInfo
)

func main() {
	flag.Parse()

	if id == "" {
		log.Error("Cluster ID must be provided with the -id flag")
		return
	}

	// TODO For internal demo purposes only. Should be removed and read peers configuration
	// from a file or custom resource.
	if hasDemoPeer {
		demoClusterPeer()
	}
	clusterInfo = demoClusterInfo()

	// Setting up an in-memory config store for the agent
	store = mcmodel.MakeMCStore(memory.Make(mcmodel.MultiClusterConfigTypes))

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
			log.Debug("Reconciling..")
			added, modified, err := reconcile.AddMulticlusterConfig(store, config, clusterInfo)
			agent.PrintReconcileAddResults(added, modified, err)
		case model.EventDelete:
			log.Debugf("ServiceExpositionPolicy resource was deleted. Name: %s.%s", config.Namespace, config.Name)
			log.Debug("Deleting it from the config store..")
			store.Delete(config.Type, config.Name, config.Namespace)
			log.Debug("Reconciling..")
			deleted, err := reconcile.DeleteMulticlusterConfig(store, config, clusterInfo)
			agent.PrintReconcileDeleteResults(deleted, err)
		case model.EventUpdate:
			log.Debugf("ServiceExpositionPolicy resource was updated. Name: %s.%s", config.Namespace, config.Name)
			log.Debug("Updating it in the config store..")
			store.Update(config)
		}
		log.Debugf("Config store now has %d ServiceExpositionPolicy entries", len(store.ServiceExpositionPolicies()))
	})

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	stopCh := make(chan struct{})

	log.Debug("Starting controller..")
	go ctl.Run(stopCh)

	log.Debugf("Starting agent listener on port %d..", port)
	server, err := agent.NewServer("localhost", uint16(port), store)
	go server.Run()

	log.Debugf("Starting agent clients. Number of peers: %d", len(peers))
	clients := []*agent.Client{}
	for _, peer := range peers {
		client, err := agent.NewClient(id, peer, cl, &store, clusterInfo)
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

func demoClusterInfo() agent.DebugClusterInfo {
	return agent.DebugClusterInfo{
		IPs: map[string]string{
			"clusterA": "127.0.0.1",
			"clusterB": "127.0.0.1",
		},
		Ports: map[string]uint32{
			"clusterA": 80,
			"clusterB": 80,
		},
	}
}

func demoClusterPeer() {
	peer := agent.PeerAgent{
		ID:      "clusterB",
		Address: "localhost",
		Port:    8999,
	}
	peers = append(peers, peer)
}

func init() {
	// set up Istio logger
	o := log.DefaultOptions()
	o.SetOutputLevel(log.DefaultScopeName, log.DebugLevel)
	if err := log.Configure(o); err != nil {
		fmt.Printf("Failed to configure logger: %v", err)
		return
	}

	flag.StringVar(&id, "id", "", "Required. Cluster ID where the agent is running.")
	flag.IntVar(&port, "port", 8999, "Listen port for the agent to listen on.")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&context, "context", "", "Kubeconfig context to be used. Only required if out-of-cluster.")
	flag.StringVar(&namespace, "namespace", "", "Namespace to watch. Default (or empty string) is all namespaces.")

	flag.BoolVar(&hasDemoPeer, "hasDemoPeer", false, "Demo purposes only. To be removed.")

	peers = []agent.PeerAgent{}
}
