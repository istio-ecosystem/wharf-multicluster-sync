// Licensed Materials - Property of IBM
// (C) Copyright IBM Corp. 2018. All Rights Reserved.
// US Government Users Restricted Rights - Use, duplication or
// disclosure restricted by GSA ADP Schedule Contract with IBM Corp.
// Copyright 2018 IBM Corporation

package crd

import (
	"encoding/json"
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

	"github.com/istio-ecosystem/wharf-multicluster-sync/multicluster/pkg/agent"
	mcmodel "github.com/istio-ecosystem/wharf-multicluster-sync/multicluster/pkg/model"

	multierror "github.com/hashicorp/go-multierror"

	kube_v1 "k8s.io/api/core/v1"
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
		return fmt.Errorf("expected %#v, parsed %#v", typs, typsParsed)
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
func readAndConvert(reader io.Reader, writer io.Writer, clusterConfig *agent.ClusterConfig) error {
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

	istioConfigs, err := mcmodel.ConvertBindingsAndExposuresEgressIngress(configs, clusterConfig)
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
			return fmt.Errorf("unknown kind %q for %v", istiocrd.ResourceName(istioConfig.Type), istioConfig.Name)
		}

		err = schema.Validate(istioConfig.Name, istioConfig.Namespace, istioConfig.Spec)
		if err != nil {
			return multierror.Prefix(err, fmt.Sprintf("generated %s %s/%s does not validate",
				istioConfig.Type, istioConfig.Namespace, istioConfig.Name))
		}
	}

	err = writeIstioYAMLOutput(configDescriptor, istioConfigs, writer)
	return err
}

func configToIstioObj(descriptor istiomodel.ConfigDescriptor, config istiomodel.Config) (IstioObject, error) {
	// Does config wrap an Istio object?
	schema, exists := descriptor.GetByType(config.Type)
	if !exists {
		return nil, fmt.Errorf("unknown kind %q for %v", istiocrd.ResourceName(config.Type), config.Name)
	}

	// the memory ConfigStore uses the Date as the ResourceVersion making testing nonrepeatable
	config.ResourceVersion = ""

	// TODO Check for MC model object

	obj, err := istiocrd.ConvertConfig(schema, config)
	if err != nil {
		return nil, fmt.Errorf("could not decode %v: %v", config.Name, err)
	}

	return obj, nil
}

func writeIstioYAMLOutput(descriptor istiomodel.ConfigDescriptor, configs []istiomodel.Config, writer io.Writer) error {
	for i, config := range configs {
		obj, err := configToIstioObj(descriptor, config)
		if err != nil {
			log.Errorf("could not convert %v to Istio Object: %v", config, err)
			continue
		}
		bytes, err := yaml.Marshal(obj)
		if err != nil {
			log.Errorf("could not convert %v to YAML: %v", config, err)
			continue
		}
		writer.Write(bytes) // nolint: errcheck
		if i+1 < len(configs) {
			writer.Write([]byte("---\n")) // nolint: errcheck
		}
	}

	return nil
}

func writeK8sYAMLOutput(svcs []kube_v1.Service, writer io.Writer) error {
	for i, svc := range svcs {
		bytes, err := yaml.Marshal(svc)
		if err != nil {
			log.Errorf("Could not convert %v to YAML: %v", svc, err)
			continue
		}
		writer.Write(bytes) // nolint: errcheck
		if i+1 < len(svcs) {
			writer.Write([]byte("---\n")) // nolint: errcheck
		}
	}

	return nil
}

// loadConfig will load the cluster configuration from the provided JSON file
func loadConfig(file string) (*agent.ClusterConfig, error) {
	jsonFile, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close() // nolint: errcheck

	var config agent.ClusterConfig
	bytes, _ := ioutil.ReadAll(jsonFile)
	err = json.Unmarshal(bytes, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
