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

	multierror "github.com/hashicorp/go-multierror"

	istiocrd "istio.io/istio/pilot/pkg/config/kube/crd"
	"istio.io/istio/pilot/pkg/config/memory"
	istiomodel "istio.io/istio/pilot/pkg/model"

	"istio.io/istio/pilot/test/util"

	"github.com/istio-ecosystem/wharf-multicluster-sync/multicluster/pkg/agent"
	mcmodel "github.com/istio-ecosystem/wharf-multicluster-sync/multicluster/pkg/model"

	kube_v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

func TestBindingToDirectIngressConfiguration(t *testing.T) {
	tt := []struct {
		config   string // Config of binding cluster
		in       string // Filename of SEP and RSBs
		store    string // Filename for baseline Istio configuration for merging
		svcStore string // Filename for baseline Kuberentes services configuration for merging
		out      string // Filename for generated Istio configuration
	}{
		{config: "cluster1.json",
			in:  "rshriram-demo-binding.yaml",
			out: "banix-demo-binding.yaml"},
		{config: "cluster_a.json",
			in:  "rshriram-demo-exposure.yaml",
			out: "banix-demo-exposure.yaml"},
		{config: "cluster1.json",
			in:  "reviews-binding.yaml",
			out: "reviews-directingress-binding.yaml"},
		{config: "cluster1.json",
			in:  "reviews-binding-v1-only.yaml",
			out: "reviews-directingress-binding-v1-only.yaml"},
		{config: "cluster_a.json",
			in:    "reviews-exposure-both.yaml",
			out:   "reviews-directingress-exposure.yaml",
			store: "reviews-exposure-starter.yaml"},
		{config: "cluster_a.json",
			in:  "reviews-binding-three-versions.yaml",
			out: "reviews-binding-three-versions.yaml"},
		{config: "cluster_a.json",
			in:  "ratings-binding.yaml",
			out: "ratings-binding.yaml"},
		{config: "cluster_b_listens_cd.json",
			in:  "ratings-binding-both-cd.yaml",
			out: "ratings-binding-both-cd.yaml"},
	}

	for _, tc := range tt {
		t.Run(tc.in, func(t *testing.T) {
			clusterConfig, err := loadConfig("../../../test/mc-agent/" + tc.config)
			if err != nil {
				t.Fatal(err)
			}

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

			var svcStore []kube_v1.Service
			if tc.svcStore != "" {
				svcStore, err = createTestServiceStoreFromFile("../../../test/expose-binding/" + tc.svcStore)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				svcStore = make([]kube_v1.Service, 0)
			}

			if err := readAndConvertDirectIngress(in, out, clusterConfig, store, svcStore); err != nil {
				t.Fatalf("Unexpected error converting configs: %v", err)
			}

			util.CompareYAML(outFilename, t)
		})
	}
}

// readAndConvertDirectIngress converts a .yaml file of ServiceExposurePolicy and RemoteServiceBinding to Istio config .yaml file
func readAndConvertDirectIngress(reader io.Reader, writer io.Writer, clusterConfig *agent.ClusterConfig, store istiomodel.ConfigStore, svcStore []kube_v1.Service) error { // nolint: lll
	configs, err := readConfigs(reader)
	if err != nil {
		return err
	}

	// Ensure every input config is valid
	mcDescriptor := istiomodel.ConfigDescriptor{
		mcmodel.ServiceExpositionPolicy,
		mcmodel.RemoteServiceBinding,
	}
	for _, config := range configs {
		schema, exists := mcDescriptor.GetByType(config.Type)
		if !exists {
			continue
		}
		err = schema.Validate(config.Name, config.Namespace, config.Spec)
		if err != nil {
			return multierror.Prefix(err, "input validation failure")
		}
	}

	istioConfigs, svcs, err := mcmodel.ConvertBindingsAndExposuresDirectIngress(configs, clusterConfig, store, svcStore)
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
			continue // Don't validate generated K8s config (the Service)
		}

		err = schema.Validate(istioConfig.Name, istioConfig.Namespace, istioConfig.Spec)
		if err != nil {
			return multierror.Prefix(err, "validation failure")
		}
	}

	err = writeIstioYAMLOutput(configDescriptor, istioConfigs, writer)
	if err != nil {
		return multierror.Prefix(err, "couldn't write yaml")
	}

	if len(svcs) > 0 {
		writer.Write([]byte("---\n")) // nolint: errcheck
		err = writeK8sYAMLOutput(svcs, writer)
		if err != nil {
			return multierror.Prefix(err, "couldn't write yaml")
		}
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

func createTestServiceStoreFromFile(fname string) ([]kube_v1.Service, error) {
	svcs := []kube_v1.Service{}

	if fname != "" {
		reader, err := os.Open(fname)
		if err != nil {
			return nil, err
		}
		defer reader.Close() // nolint: errcheck

		svcs, err = readK8sServices(reader)
		if err != nil {
			return nil, err
		}
	}

	return svcs, nil
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

func readK8sServices(reader io.Reader) ([]kube_v1.Service, error) {
	outSvcs := make([]kube_v1.Service, 0)

	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	runtimeScheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(runtimeScheme)
	deserializer := codecs.UniversalDeserializer()
	obj, _, err := deserializer.Decode(data, nil, nil)
	if err != nil {
		return nil, err
	}

	// now use switch over the type of the object
	// and match each type-case
	switch o := obj.(type) {
	case *kube_v1.Service:
		outSvcs = append(outSvcs, *o)
	default:
		fmt.Printf("Unexpected Kubernetes type %v\n", o)
	}

	return outSvcs, nil
}
