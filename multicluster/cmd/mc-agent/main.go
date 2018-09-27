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

	"gopkg.in/yaml.v2"

	"istio.io/istio/pilot/pkg/config/kube/crd"

	"github.ibm.com/istio-research/multicluster-roadmap/multicluster/pkg/agent"
	mccrd "github.ibm.com/istio-research/multicluster-roadmap/multicluster/pkg/config/kube/crd"
	mcmodel "github.ibm.com/istio-research/multicluster-roadmap/multicluster/pkg/model"

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
	configYAML string

	mcStore       mcmodel.MCConfigStore
	istioStore    model.ConfigStore
	clusterConfig *agent.ClusterConfig

	configsMgmt *agent.ConfigsManagement
)

func main() {
	flag.Parse()

	// Load the cluster config from the provided json or yaml file
	var err error
	if configJSON != "" {
		clusterConfig, err = loadJsonConfig(configJSON)
	} else if configYAML != "" {
		clusterConfig, err = loadYamlConfig(configYAML)
	} else {
		err = fmt.Errorf("Cluster configuration file must be provided with the -configJson or -configYaml flag")
	}

	if err != nil {
		log.Errora(err)
		return
	}

	// Set up a Kubernetes API ConfigStore for Istio configs
	istioStore, err := makeKubeConfigIstioController()
	if err != nil {
		log.Errora(err)
		return
	}

	configsMgmt = agent.NewConfigsManagement(kubeconfig, context, istioStore, clusterConfig)
	if configsMgmt == nil {
		log.Error("Failed to create an instance of ConfigsManagement")
		return
	}

	// Set up a store wrapper for the Multi-Cluster controller
	desc := model.ConfigDescriptor{mcmodel.ServiceExpositionPolicy, mcmodel.RemoteServiceBinding}
	cl, err := mccrd.NewClient(kubeconfig, context, desc, namespace)
	if err != nil {
		log.Errora(err)
		return
	}

	// Register the Multi-Cluster CRDs if they aren't already registered
	// err = cl.RegisterResources()
	// if err != nil {
	// 	log.Errora(err)
	// 	return
	// }

	// Setting up a controller for the configured namespace to periodically watch for changes
	ctl := mccrd.NewController(cl, kube.ControllerOptions{WatchedNamespace: namespace, ResyncPeriod: resyncPeriod})

	// Register model configs event handler that will update the config store accordingly
	// for ServiceExpositionPolicy resources
	ctl.RegisterEventHandler(mcmodel.ServiceExpositionPolicy.Type, func(config model.Config, ev model.Event) {
		switch ev {
		case model.EventAdd:
			log.Debugf("ServiceExpositionPolicy resource was added. Name: %s.%s", config.Namespace, config.Name)
			configsMgmt.McConfigAdded(config)
		case model.EventDelete:
			log.Debugf("ServiceExpositionPolicy resource was deleted. Name: %s.%s", config.Namespace, config.Name)
			configsMgmt.McConfigDeleted(config)
		case model.EventUpdate:
			log.Debugf("ServiceExpositionPolicy resource was updated. Name: %s.%s", config.Namespace, config.Name)
			configsMgmt.McConfigModified(config)
		}
		log.Debugf("Config store now has %d ServiceExpositionPolicy entries", len(mcStore.ServiceExpositionPolicies()))
	})

	// Register model configs event handler that will update the config store accordingly
	// for RemoteServiceBinding resources
	ctl.RegisterEventHandler(mcmodel.RemoteServiceBinding.Type, func(config model.Config, ev model.Event) {
		configKey := fmt.Sprintf("%s.%s", config.Namespace, config.Name)
		connMode := config.Labels[agent.ConnectionModeKey]
		switch ev {
		case model.EventAdd:
			log.Debugf("RemoteServiceBinding resource was added. Key: %s Mode: %s", configKey, connMode)
			if connMode == agent.ConnectionModeLive {
				configsMgmt.McConfigAdded(config)
			}
		case model.EventDelete:
			log.Debugf("RemoteServiceBinding resource was deleted. Key: %s Mode: %s", configKey, connMode)
			if connMode == agent.ConnectionModeLive {
				configsMgmt.McConfigDeleted(config)
			}
		case model.EventUpdate:
			log.Debugf("RemoteServiceBinding resource was updated. Key: %s Mode: %s", configKey, connMode)
			if connMode == agent.ConnectionModeLive {
				configsMgmt.McConfigModified(config)
			}
		}
		log.Debugf("Config store now has %d RemoteServiceBinding entries", len(mcStore.RemoteServiceBindings()))
	})

	// Set up a store wrapper for the Multi-Cluster controller
	mcStore = mcmodel.MakeMCStore(ctl)

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	stopCh := make(chan struct{})

	log.Debug("Starting Istio controller..")
	go istioStore.Run(stopCh)

	log.Debug("Starting Multi-Cluster controller..")
	go ctl.Run(stopCh)

	log.Debugf("Starting agent listener on port %d..", clusterConfig.AgentPort)
	server, err := agent.NewServer(clusterConfig, mcStore)
	go server.Run()

	log.Debugf("Starting agent clients. Number of peers: %d", len(clusterConfig.Peers))
	clients := []*agent.Client{}
	for _, peer := range clusterConfig.Peers {
		client, err := agent.NewClient(clusterConfig, &peer, &mcStore, istioStore)
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

func makeKubeConfigIstioController() (model.ConfigStoreCache, error) {
	configClient, err := crd.NewClient(kubeconfig, context, model.IstioConfigTypes, namespace)
	if err != nil {
		return nil, err
	}

	ctl := crd.NewController(configClient, kube.ControllerOptions{WatchedNamespace: namespace, ResyncPeriod: resyncPeriod})

	return ctl, nil
}

// loadJsonConfig will load the cluster configuration from the provided JSON file
func loadJsonConfig(file string) (*agent.ClusterConfig, error) {
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

// loadYamlConfig will load the cluster configuration from the provided YAML file
func loadYamlConfig(file string) (*agent.ClusterConfig, error) {
	yamlFile, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer yamlFile.Close()

	var config agent.ClusterConfig
	bytes, _ := ioutil.ReadAll(yamlFile)
	err = yaml.Unmarshal(bytes, &config)
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
	flag.StringVar(&configYAML, "configYaml", "", "Config YAML file to use for the agent configuration")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&context, "context", "", "Kubeconfig context to be used. Only required if out-of-cluster.")
	flag.StringVar(&namespace, "namespace", "", "Namespace to watch. Default (or empty string) is all namespaces.")
}
