// (C) Copyright IBM Corp. 2018. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/howeyc/fsnotify"

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
	config     string

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

	// Load the cluster config from the provided as a yaml file
	var err error
	if config != "" {
		clusterConfig, err = agent.LoadConfig(config)
		if err != nil {
			log.Errorf("Could not load config: %v", err)
		}
		configWatcher = launchConfigWatcher(config)
	} else {
		err = fmt.Errorf("cluster configuration file must be provided with the -config flag")
	}
	if err != nil {
		log.Errora(err)
		return
	}
	if configWatcher != nil {
		defer configWatcher.Close()
	}
	log.Debugf("Cluster Configuration: %#v", clusterConfig)

	// Set up a Kubernetes API ConfigStore for Istio configs
	istioStore, err := makeKubeConfigIstioController()
	if err != nil {
		log.Errorf("Could not make Kube Config Istio controller: %v", err)
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
		log.Errorf("Could not create MC CRD Client: %v", err)
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
		// This style is not fully supported!
		log.Warn("Using Egress/Ingress Style")
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
	// configWatcher will be closed by defer'ed func above

	_ = log.Sync()
}

func launchPeerClient(peer agent.ClusterConfig) {
	client, err := agent.NewClient(clusterConfig, &peer, &mcStore, istioStore)
	if err != nil {
		log.Errorf("Failed to create an agent client to peer: %s", peer.ID)
		return
	}
	log.Infof("Created agent client to peer: %s", peer.ID)
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

// launchConfigWatcher will launch a watcher to determine changes in the config
// file and notify relevant objects about those changes
func launchConfigWatcher(file string) *fsnotify.Watcher {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Errorf("Can't create watcher for %q: %v", file, err)
		return nil
	}

	onFileModified := func() {
		//Config file modified
		log.Debug("Config file modified. Reloading.")
		newClusterConfig, lderr := agent.LoadConfig(file)
		if lderr != nil {
			log.Error("Failed to reload the config file")
			return
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

	go func() {
		var timerC <-chan time.Time
		for {
			select {
			case <-timerC:
				timerC = nil
				onFileModified()
			case event := <-watcher.Event:
				// use a timer to debounce configuration updates
				if (event.IsModify() || event.IsCreate()) && timerC == nil {
					timerC = time.After(100 * time.Millisecond)
				}
			case werr := <-watcher.Error:
				log.Errorf("Watcher error: %v", werr)
			}
		}
	}()

	fileDir, _ := filepath.Split(file)
	err = watcher.Watch(fileDir)
	if err != nil {
		log.Errorf("Could not watch %q: %v", fileDir, err)
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

	flag.StringVar(&config, "config", "", "Config YAML file to use for the agent configuration")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&context, "context", "", "Kubeconfig context to be used. Only required if out-of-cluster.")
	flag.StringVar(&namespace, "namespace", "", "Namespace to watch. Default (or empty string) is all namespaces.")
}
