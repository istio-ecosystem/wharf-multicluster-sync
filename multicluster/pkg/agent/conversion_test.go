// Licensed Materials - Property of IBM
// (C) Copyright IBM Corp. 2018. All Rights Reserved.
// US Government Users Restricted Rights - Use, duplication or
// disclosure restricted by GSA ADP Schedule Contract with IBM Corp.
// Copyright 2018 IBM Corporation

package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"testing"

	"github.com/ghodss/yaml"
	multierror "github.com/hashicorp/go-multierror"

	istiocrd "istio.io/istio/pilot/pkg/config/kube/crd"
	"istio.io/istio/pilot/pkg/config/memory"
	istiomodel "istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pkg/log"

	"istio.io/istio/pilot/test/util"

	"github.com/istio-ecosystem/wharf-multicluster-sync/multicluster/pkg/config/kube/crd"
	mcmodel "github.com/istio-ecosystem/wharf-multicluster-sync/multicluster/pkg/model"
)

// TestServiceToBinding tests agent.exposedServicesToBinding()
func TestServiceToBinding(t *testing.T) {
	tt := []struct {
		config string            // Config of binding cluster
		in     map[string]string // Map of cluster to exposed svcs on that cluster
		out    string            // YAML of RSB(s) produced
	}{
		{config: "cluster_a.json",
			in: map[string]string{
				"cluster-b": "sample-exposure.yaml",
			},
			out: "sample-exposure.yaml"},
		{config: "cluster_a.json",
			in: map[string]string{
				"cluster-b": "rshriram-demo-exposure.yaml",
			},
			out: "rshriram-demo-exposure.yaml"},
		{config: "cluster_a.json",
			in: map[string]string{
				"cluster-b": "reviews-exposure-both.yaml",
			},
			out: "reviews-exposure.yaml"},
		{config: "cluster_b_listens_cd.json",
			in: map[string]string{
				"cluster-c": "ratings-exposure.yaml",
				"cluster-d": "ratings-exposure.yaml",
			},
			out: "ratings-exposure-both-cd.yaml"},
		// Commenting this one because no service is exposed therefore
		// no RSB is generated
		// {in: "ratings-exposure.yaml",
		// 	out: "ratings-exposure.yaml"},
	}

	for _, tc := range tt {
		t.Run(tc.out, func(t *testing.T) {
			clusterConfig, err := loadConfig("../test/mc-agent/" + tc.config)
			if err != nil {
				t.Fatal(err)
			}

			outFilename := "../test/generated/" + tc.out
			out, err := os.Create(outFilename)
			if err != nil {
				t.Fatal(err)
			}
			defer out.Close() // nolint: errcheck

			// Sort the keys so output for test purposes deterministic
			for i, peerName := range sortedStringKeys(tc.in) {
				inName := tc.in[peerName]
				in, err := os.Open("../test/expose-binding/" + inName)
				if err != nil {
					t.Fatal(err)
				}
				defer in.Close() // nolint: errcheck

				if err := readAndConvert(in, out, clusterConfig, peerName); err != nil {
					t.Fatalf("Unexpected error converting configs: %v", err)
				}
				if i+1 < len(tc.in) {
					out.Write([]byte("---\n")) // nolint: errcheck
				}
			}

			util.CompareYAML(outFilename, t)
		})
	}
}

func sortedStringKeys(in map[string]string) []string {
	out := make([]string, 0)
	for key := range in {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// readAndConvert converts a .yaml file of ServiceExposurePolicy and RemoteServiceBinding to Istio config .yaml file
func readAndConvert(reader io.Reader, writer io.Writer, clusterConfig *ClusterConfig, peerName string) error {
	peer, err := lookupPeer(peerName, clusterConfig)
	if err != nil {
		return err
	}

	configs, err := readConfigs(reader)
	if err != nil {
		return err
	}

	cs, err := createDebugMCConfigStore(configs)
	if err != nil {
		return multierror.Prefix(err, "couldn't make debug store:")
	}

	store := mcmodel.MakeMCStore(cs)
	server, err := NewServer(clusterConfig, store)
	if err != nil {
		return err
	}

	svcs := server.exposedServices(clusterConfig.ID)

	var istioStore istiomodel.ConfigStore
	client, err := NewClient(clusterConfig, peer, &store, istioStore)
	if err != nil {
		return err
	}

	exposedSvcs := ExposedServices{Services: svcs}
	binding := client.createRemoteServiceBinding(&exposedSvcs, ConnectionModeLive)
	if binding != nil {
		if err = mcmodel.RemoteServiceBinding.Validate(binding.Name, binding.Namespace, binding.Spec); err != nil {
			return multierror.Prefix(err, "validation error:")
		}

		err = writeMCYAMLOutput(mcmodel.MultiClusterConfigTypes, []istiomodel.Config{*binding}, writer)
		if err != nil {
			return err
		}
	}

	return nil
}

func writeMCYAMLOutput(descriptor istiomodel.ConfigDescriptor, configs []istiomodel.Config, writer io.Writer) error {
	for i, config := range configs {
		schema, exists := descriptor.GetByType(config.Type)
		if !exists {
			log.Errorf("Unknown kind %q for %v", istiocrd.ResourceName(config.Type), config.Name)
			continue
		}
		obj, err := crd.ConvertConfig(schema, config)
		if err != nil {
			log.Errorf("Could not decode %v: %v", config.Name, err)
			continue
		}
		bytes, err := yaml.Marshal(obj)
		if err != nil {
			log.Errorf("Could not convert %v to YAML: %v", config, err)
			continue
		}
		writer.Write(bytes) // nolint: errcheck
		if i+1 < len(configs) {
			writer.Write([]byte("---\n")) // nolint: errcheck
		}
	}

	return nil
}

func lookupPeer(peerName string, clusterConfig *ClusterConfig) (*ClusterConfig, error) {
	for _, peer := range clusterConfig.WatchedPeers {
		if peer.ID == peerName {
			return &peer, nil
		}
	}

	return nil, fmt.Errorf("no peer %q in %v", peerName, clusterConfig)
}

func readConfigs(reader io.Reader) ([]istiomodel.Config, error) {

	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	config, _, err := crd.ParseInputs(string(data))
	if err != nil {
		return nil, err
	}

	return config, nil
}

func createDebugMCConfigStore(configs []istiomodel.Config) (istiomodel.ConfigStore, error) {
	out := memory.Make(mcmodel.MultiClusterConfigTypes)
	for _, config := range configs {
		_, err := out.Create(config)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// loadConfig will load the cluster configuration from the provided JSON file
func loadConfig(file string) (*ClusterConfig, error) {
	jsonFile, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close() // nolint: errcheck

	var config ClusterConfig
	bytes, _ := ioutil.ReadAll(jsonFile)
	err = json.Unmarshal(bytes, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
