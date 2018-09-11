// Licensed Materials - Property of IBM
// (C) Copyright IBM Corp. 2018. All Rights Reserved.
// US Government Users Restricted Rights - Use, duplication or
// disclosure restricted by GSA ADP Schedule Contract with IBM Corp.
// Copyright 2018 IBM Corporation

package crd

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"

	istiocrd "istio.io/istio/pilot/pkg/config/kube/crd"
	"istio.io/istio/pilot/pkg/config/memory"
	istiomodel "istio.io/istio/pilot/pkg/model"

	"istio.io/istio/pilot/test/util"

	"github.ibm.com/istio-research/multicluster-roadmap/multicluster/pkg/model"
)

func TestBindingToSNIConfiguration(t *testing.T) {
	tt := []struct {
		in    string // Filename of SEP and RSBs
		store string // Filename for baseline Istio configuration for merging
		out   string // Filename for generated Istio configuration
	}{
		{in: "rshriram-demo-binding.yaml",
			out: "banix-demo-binding.yaml"},
		{in: "rshriram-demo-exposure.yaml",
			out: "banix-demo-exposure.yaml"},
		{in: "reviews-binding.yaml",
			out: "reviews-sni-binding.yaml"},
		// TODO restore
		//		{in: "reviews-binding-v1-only.yaml",
		//			out: "reviews-sni-binding-v1-only.yaml"},
		{in: "reviews-exposure.yaml",
			out:   "reviews-sni-exposure.yaml",
			store: "reviews-exposure-starter.yaml"},
	}

	for _, tc := range tt {
		t.Run(tc.in, func(t *testing.T) {
			in, err := os.Open("../../../test/expose-binding/" + tc.in)
			if err != nil {
				t.Fatal(err)
			}
			defer in.Close() // nolint: errcheck

			outFilename := "../../../test/istio-expose-binding/" + tc.out
			out, err := os.Create(outFilename)
			if err != nil {
				t.Fatal(err)
			}
			defer out.Close() // nolint: errcheck

			var store istiomodel.ConfigStore
			if tc.store != "" {
				store, err = createTestConfigStoreFromFile("../../../test/expose-binding/" + tc.store)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				store, _ = createTestConfigStore([]istiomodel.Config{})
			}

			if err := readAndConvertSNI(in, out, store); err != nil {
				t.Fatalf("Unexpected error converting configs: %v", err)
			}

			util.CompareYAML(outFilename, t)
		})
	}
}

// readAndConvertSNI converts a .yaml file of ServiceExposurePolicy and RemoteServiceBinding to Istio config .yaml file
func readAndConvertSNI(reader io.Reader, writer io.Writer, store istiomodel.ConfigStore) error {
	configs, err := readConfigs(reader)
	if err != nil {
		return err
	}

	ci := debugClusterInfo{
		ips: map[string]string{
			"clusterC": "127.0.0.1",
			"cluster2": "127.0.0.1",
		},
		ports: map[string]uint32{
			"clusterC": 80,
			"cluster2": 80,
		},
	}
	istioConfigs, err := model.ConvertBindingsAndExposuresDirectIngress(configs, ci, store)
	if err != nil {
		return err
	}

	configDescriptor := istiomodel.ConfigDescriptor{
		istiomodel.VirtualService,
		istiomodel.Gateway,
		istiomodel.DestinationRule,
		istiomodel.ServiceEntry,
	}

	// Ensure every generated config is valid
	for _, istioConfig := range istioConfigs {
		schema, exists := configDescriptor.GetByType(istioConfig.Type)
		if !exists {
			return fmt.Errorf("Unknown kind %q for %v", istiocrd.ResourceName(istioConfig.Type), istioConfig.Name)
		}

		err = schema.Validate(istioConfig.Name, istioConfig.Namespace, istioConfig.Spec)
		if err != nil {
			return err
		}
	}

	err = writeIstioYAMLOutput(configDescriptor, istioConfigs, writer)
	if err != nil {
		return err
	}

	return nil
}

func createTestConfigStoreFromFile(fname string) (istiomodel.ConfigStore, error) {
	configs := []istiomodel.Config{}

	if fname != "" {
		reader, err := os.Open(fname)
		if err != nil {
			return nil, err
		}
		defer reader.Close() // nolint: errcheck

		configs, err = readIstioConfigs(reader)
		if err != nil {
			return nil, err
		}
	}

	return createTestConfigStore(configs)
}

func createTestConfigStore(configs []istiomodel.Config) (istiomodel.ConfigStore, error) {
	out := memory.Make(istiomodel.IstioConfigTypes)
	for _, config := range configs {
		_, err := out.Create(config)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func readIstioConfigs(reader io.Reader) ([]istiomodel.Config, error) {

	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	config, _, err := istiocrd.ParseInputs(string(data))
	if err != nil {
		return nil, err
	}

	return config, nil
}
