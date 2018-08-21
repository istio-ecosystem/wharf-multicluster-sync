package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"istio.io/istio/pilot/pkg/config/memory"
	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pilot/pkg/serviceregistry/kube"
	"istio.io/istio/pkg/log"

	"github.ibm.com/istio-research/multicluster-roadmap/multicluster/pkg/config/kube/crd"
	mcmodel "github.ibm.com/istio-research/multicluster-roadmap/multicluster/pkg/model"

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

	store mcmodel.MCConfigStore
)

func main() {
	flag.Parse()

	// Setting up an in-memory config store for the agent
	store = mcmodel.MakeMCStore(memory.Make(mcmodel.MultiClusterConfigTypes))

	// Set up a Kubernetes API client for the Multi-Cluster configs
	desc := model.ConfigDescriptor{mcmodel.ServiceExpositionPolicy}
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
		case model.EventDelete:
			log.Debugf("ServiceExpositionPolicy resource was deleted. Name: %s.%s", config.Namespace, config.Name)
			log.Debug("Deleting it from the config store..")
			store.Delete(config.Type, config.Name, config.Namespace)
		case model.EventUpdate:
			log.Debugf("ServiceExpositionPolicy resource was updated. Name: %s.%s", config.Namespace, config.Name)
			log.Debug("Updating it in the config store..")
			store.Update(config)
		}
		log.Debugf("Config store now has %d ServiceExpositionPolicy entries", len(store.ServiceExpositionPolicies()))
	})

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	stopCtrl := make(chan struct{})

	log.Info("Controller started")
	go ctl.Run(stopCtrl)

	<-shutdown
	close(stopCtrl)
}

func init() {
	// set up Istio logger
	o := log.DefaultOptions()
	o.SetOutputLevel(log.DefaultScopeName, log.DebugLevel)
	if err := log.Configure(o); err != nil {
		fmt.Printf("Failed to configure logger: %v", err)
		return
	}

	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&context, "context", "", "Kubeconfig context to be used. Only required if out-of-cluster.")
	flag.StringVar(&namespace, "namespace", "", "Namespace to watch. Default (or empty string) is all namespaces.")
}
