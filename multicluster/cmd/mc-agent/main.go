package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

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
)

func main() {
	flag.Parse()

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

	ctl := crd.NewController(cl, kube.ControllerOptions{WatchedNamespace: namespace, ResyncPeriod: resyncPeriod})

	ctl.RegisterEventHandler(mcmodel.ServiceExpositionPolicy.Type, func(config model.Config, ev model.Event) {
		switch ev {
		case model.EventAdd:
			log.Infof("ServiceExpositionPolicy resource was added. Name: %s.%s", config.Namespace, config.Name)
		case model.EventDelete:
			log.Infof("ServiceExpositionPolicy resource was deleted. Name: %s.%s", config.Namespace, config.Name)
		case model.EventUpdate:
			log.Infof("ServiceExpositionPolicy resource was updated. Name: %s.%s", config.Namespace, config.Name)
		}
	})

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	stopCtrl := make(chan struct{})

	go ctl.Run(stopCtrl)

	<-shutdown
	close(stopCtrl)
}

func init() {
	// set up Istio logger
	if err := log.Configure(log.DefaultOptions()); err != nil {
		fmt.Printf("Failed to configure logger: %v", err)
		return
	}

	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&context, "context", "", "Kubeconfig context to be used. Only required if out-of-cluster.")
	flag.StringVar(&namespace, "namespace", "", "Namespace to watch. Default (or empty string) is all namespaces.")
}
