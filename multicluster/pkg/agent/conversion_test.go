// Licensed Materials - Property of IBM
// (C) Copyright IBM Corp. 2018. All Rights Reserved.
// US Government Users Restricted Rights - Use, duplication or
// disclosure restricted by GSA ADP Schedule Contract with IBM Corp.
// Copyright 2018 IBM Corporation

package agent

import (
	"io"
	"io/ioutil"
	"os"
	"testing"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/ghodss/yaml"

	istiocrd "istio.io/istio/pilot/pkg/config/kube/crd"
	"istio.io/istio/pilot/pkg/config/memory"
	istiomodel "istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pkg/log"

	"istio.io/istio/pilot/test/util"

	mcmodel "github.ibm.com/istio-research/multicluster-roadmap/multicluster/pkg/model"
	"github.ibm.com/istio-research/multicluster-roadmap/multicluster/pkg/config/kube/crd"
)

// TestServiceToBinding tests agent.exposedServicesToBinding()
func TestServiceToBinding(t *testing.T) {
	tt := []struct {
		in  string
		out string
	}{
		{in: "sample-exposure.yaml",
			out: "sample-exposure.yaml"},
		{in: "rshriram-demo-exposure.yaml",
			out: "rshriram-demo-exposure.yaml"},
		{in: "reviews-exposure.yaml",
			out: "reviews-exposure.yaml"},
		{in: "ratings-exposure.yaml",
			out: "ratings-exposure.yaml"},
	}

	for _, tc := range tt {
		t.Run(tc.in, func(t *testing.T) {
			in, err := os.Open("../test/expose-binding/" + tc.in)
			if err != nil {
				t.Fatal(err)
			}
			defer in.Close() // nolint: errcheck

			outFilename := "../test/generated/" + tc.out
			out, err := os.Create(outFilename)
			if err != nil {
				t.Fatal(err)
			}
			defer out.Close() // nolint: errcheck

			if err := readAndConvert(in, out, "clusterA", "127.0.0.1", 8080); err != nil {
				t.Fatalf("Unexpected error converting configs: %v", err)
			}

			util.CompareYAML(outFilename, t)
		})
	}
}

// readAndConvert converts a .yaml file of ServiceExposurePolicy and RemoteServiceBinding to Istio config .yaml file
func readAndConvert(reader io.Reader, writer io.Writer, clusterID string, addr string, port uint16) error {
	configs, err := readConfigs(reader)
	if err != nil {
		return err
	}

	cs, err := createDebugMCConfigStore(configs)
	if err != nil {
		return multierror.Prefix(err, "couldn't make debug store:")
	}

    store := mcmodel.MakeMCStore(cs)
	server, err := NewServer(addr, port, store)
	if err != nil {
		return err
	}

	svcs := server.exposedServices(clusterID)

	var config ClusterConfig
	var peer ClusterConfig
	var istioStore istiomodel.ConfigStore
	var crdClient crd.Client
	_ = configs
	client, err := NewClient(&config, &peer, &crdClient, &store, istioStore)
	if err != nil {
		return err
	}

	exposedSvcs := ExposedServices{ Services: svcs }
	binding := client.exposedServicesToBinding(&exposedSvcs)

	err = writeMCYAMLOutput(mcmodel.MultiClusterConfigTypes, []istiomodel.Config { *binding }, writer)
	if err != nil {
		return err
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
