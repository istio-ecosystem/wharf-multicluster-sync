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
)

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
		// Verify we can convert Kubernetes Istio Ingress
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

func writeYAMLOutput(descriptor istiomodel.ConfigDescriptor, configs []istiomodel.Config, writer io.Writer) {
	for i, config := range configs {
		schema, exists := descriptor.GetByType(config.Type)
		if !exists {
			log.Errorf("Unknown kind %q for %v", istiocrd.ResourceName(config.Type), config.Name)
			continue
		}
		obj, err := ConvertConfig(schema, config)
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
}
