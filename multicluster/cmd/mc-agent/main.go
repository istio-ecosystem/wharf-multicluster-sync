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

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v2"

	"istio.io/istio/pilot/pkg/config/kube/crd"

	"github.com/istio-ecosystem/wharf-multicluster-sync/multicluster/pkg/agent"
	mccrd "github.com/istio-ecosystem/wharf-multicluster-sync/multicluster/pkg/config/kube/crd"
	mcmodel "github.com/istio-ecosystem/wharf-multicluster-sync/multicluster/pkg/model"

	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pilot/pkg/serviceregistry/kube"
	"istio.io/istio/pkg/log"

	// Importing the API messages so that when resource events are fired the
	// resource will be parsed into a message object
	_ "github.com/istio-ecosystem/wharf-multicluster-sync/api/multicluster/v1alpha1"
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
	configWatcher *fsnotify.Watcher
	clientsCfgCh  map[string](chan agent.ClusterConfig)
	stopCh        chan struct{}

	configsMgmt *agent.ConfigsManagement
)

func main() {
	flag.Parse()

	// Load the cluster config from the provided json or yaml file
	var err error
	if configJSON != "" {
		clusterConfig, err = loadConfig(configJSON, false)
		configWatcher = launchConfigWatcher(configJSON, false)
	} else if configYAML != "" {
		clusterConfig, err = loadConfig(configYAML, true)
		configWatcher = launchConfigWatcher(configYAML, true)
	} else {
		err = fmt.Errorf("cluster configuration file must be provided with the -configJson or -configYaml flag")
	}
	if err != nil {
		log.Errora(err)
		return
	}
	if configWatcher != nil {
		defer configWatcher.Close()
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

	if os.Getenv(mcmodel.IstioConversionStyleKey) == mcmodel.DirectIngressStyle {
		log.Info("Using Direct Ingress Style")
	} else {
		log.Info("Using Egress/Ingress Style")
	}

	// Set up a store wrapper for the Multi-Cluster controller
	mcStore = mcmodel.MakeMCStore(ctl)

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	stopCh = make(chan struct{})

	log.Debug("Starting Istio controller..")
	go istioStore.Run(stopCh)

	log.Debug("Starting Multi-Cluster controller..")
	go ctl.Run(stopCh)

	log.Debugf("Starting agent listener on port %d..", clusterConfig.AgentPort)
	server, err := agent.NewServer(clusterConfig, mcStore)
	if err != nil {
		log.Errora(err)
		return
	}
	go server.Run()

	log.Debugf("Starting agent clients. Number of peers: %d", len(clusterConfig.WatchedPeers))
	clientsCfgCh = map[string]chan agent.ClusterConfig{}
	for _, peer := range clusterConfig.WatchedPeers {
		launchPeerClient(peer)
	}

	<-shutdown
	log.Debug("Shutting down the Multi-Cluster agent")

	close(stopCh)
	server.Close()

	_ = log.Sync()
}

func launchPeerClient(peer agent.ClusterConfig) {
	client, err := agent.NewClient(clusterConfig, &peer, &mcStore, istioStore)
	if err != nil {
		log.Errorf("Failed to create an agent client to peer: %s", peer.ID)
	}
	cfgCh := make(chan agent.ClusterConfig)
	clientsCfgCh[peer.ID] = cfgCh
	go client.Run(cfgCh, stopCh)
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
func loadConfig(filename string, isYaml bool) (*agent.ClusterConfig, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config agent.ClusterConfig
	bytes, _ := ioutil.ReadAll(file)
	if isYaml {
		err = yaml.Unmarshal(bytes, &config)
	} else {
		err = json.Unmarshal(bytes, &config)
	}
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func launchConfigWatcher(file string, isYaml bool) *fsnotify.Watcher {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Errora(err)
		return nil
	}
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					//Config file modified
					log.Debug("Config file modified. Reloading.")
					newClusterConfig, lderr := loadConfig(file, isYaml)
					if lderr != nil {
						log.Error("Failed to reload the config file")
						continue
					}

					clusterConfig = newClusterConfig

					handled := map[string]bool{}
					for _, peer := range clusterConfig.WatchedPeers {
						// Update client with updated configuration
						if clientsCfgCh[peer.ID] != nil {
							clientsCfgCh[peer.ID] <- *clusterConfig
						} else {
							//This is a new peer. Launch a new client for it
							launchPeerClient(peer)
						}
						handled[peer.ID] = true
					}

					//Find clients which are no longer needed and close them
					for id, cfgCh := range clientsCfgCh {
						if !handled[id] {
							clientsCfgCh[id] = nil
							close(cfgCh)
						}
					}
				}
			}
		}
	}()
	err = watcher.Add(file)
	if err != nil {
		log.Errora(err)
		return nil
	}
	return watcher
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
