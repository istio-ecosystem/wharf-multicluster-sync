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
	"reflect"
	"testing"

	"github.com/ghodss/yaml"

	istiocrd "istio.io/istio/pilot/pkg/config/kube/crd"
	istiomodel "istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pkg/log"

	"github.ibm.com/istio-research/multicluster-roadmap/multicluster/pkg/model"

	multierror "github.com/hashicorp/go-multierror"
)

// debugClusterInfo simulates the function of K8s Cluster Registry
// https://github.com/kubernetes/cluster-registry in unit tests.
type debugClusterInfo struct {
	ips   map[string]string
	ports map[string]uint32
}

var (
	// TODO This goes away if we become part of Istio
	unknownKinds = map[string]istiomodel.ProtoSchema{
		"ServiceExpositionPolicy": model.ServiceExpositionPolicy,
		"RemoteServiceBinding":    model.RemoteServiceBinding,
	}
)

func TestParseYaml(t *testing.T) {
	tt := []struct {
		in       string
		outtypes []string
	}{
		{in: "ratings-exposure.yaml",
			outtypes: []string{"*v1alpha1.ServiceExpositionPolicy"}},
		{in: "sample-exposure.yaml",
			outtypes: []string{"*v1alpha1.ServiceExpositionPolicy"}},
		{in: "ratings-binding.yaml",
			outtypes: []string{"*v1alpha1.RemoteServiceBinding"}},
		{in: "sample-binding.yaml",
			outtypes: []string{"*v1alpha1.RemoteServiceBinding"}},
		// TODO Enable ports on SEP
		// {in: "multi-port-exposure.yaml",
		//	outtypes: []string { "*v1alpha1.ServiceExpositionPolicy" } },
		// {in: "cassandra-exposure.yaml",
		//	outtypes: []string { "*v1alpha1.ServiceExpositionPolicy" } },
	}

	for _, tc := range tt {
		t.Run(tc.in, func(t *testing.T) {
			in, err := os.Open("../../../test/expose-binding/" + tc.in)
			if err != nil {
				t.Fatal(err)
			}
			defer in.Close() // nolint: errcheck

			if err := checkParsedTypes(in, tc.outtypes); err != nil {
				t.Fatalf("Unexpected error converting configs: %v", err)
			}
		})
	}
}

// checkParsedTypes verifies the input YAML file contains the expected types and not maps of interfaces
func checkParsedTypes(reader io.Reader, typs []string) error {
	configs, err := readConfigs(reader)
	if err != nil {
		return err
	}

	typsParsed := make([]string, len(configs))
	for i, config := range configs {
		typsParsed[i] = reflect.TypeOf(config.Spec).String()
	}

	if !reflect.DeepEqual(typs, typsParsed) {
		return fmt.Errorf("Expected %#v, parsed %#v\n", typs, typsParsed)
	}

	return nil
}

func readConfigs(reader io.Reader) ([]istiomodel.Config, error) {

	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	config, _, err := ParseInputs(string(data))
	if err != nil {
		return nil, err
	}

	return config, nil
}

func TestValidation(t *testing.T) {
	tt := []struct {
		in       string
		mustFail bool
	}{
		{in: "invalid-exposure.yaml",
			mustFail: true},
	}

	for _, tc := range tt {
		t.Run(tc.in, func(t *testing.T) {
			in, err := os.Open("../../../test/expose-binding/" + tc.in)
			if err != nil {
				t.Fatal(err)
			}
			defer in.Close() // nolint: errcheck

			_, err = readConfigs(in)
			if tc.mustFail {
				if err == nil {
					t.Errorf("Validated correct; failure expected")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error converting configs: %v", err)
				}
			}
		})
	}
}

// readAndConvert converts a .yaml file of ServiceExposurePolicy and RemoteServiceBinding to Istio config .yaml file
func readAndConvert(reader io.Reader, writer io.Writer) error {
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
	istioConfigs, err := model.ConvertBindingsAndExposuresEgressIngress(configs, ci)
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
			return multierror.Prefix(err, fmt.Sprintf("Generated %s %s/%s does not validate",
				istioConfig.Type, istioConfig.Namespace, istioConfig.Name))
		}
	}

	err = writeIstioYAMLOutput(configDescriptor, istioConfigs, writer)
	if err != nil {
		return err
	}

	return nil
}

func writeIstioYAMLOutput(descriptor istiomodel.ConfigDescriptor, configs []istiomodel.Config, writer io.Writer) error {
	for i, config := range configs {
		schema, exists := descriptor.GetByType(config.Type)
		if !exists {
			log.Errorf("Unknown kind %q for %v", istiocrd.ResourceName(config.Type), config.Name)
			continue
		}

		// the memory ConfigStore uses the Date as the ResourceVersion making testing difficult
		config.ResourceVersion = ""

		obj, err := istiocrd.ConvertConfig(schema, config)
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

func (ci debugClusterInfo) Ip(name string) string {
	out, ok := ci.ips[name]
	if ok {
		return out
	}
	return "255.255.255.255" // dummy value for unknown clusters
}

func (ci debugClusterInfo) Port(name string) uint32 {
	out, ok := ci.ports[name]
	if ok {
		return out
	}
	return 8080 // dummy value for unknown clusters
}
