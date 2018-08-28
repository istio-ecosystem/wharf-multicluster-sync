// Licensed Materials - Property of IBM
// (C) Copyright IBM Corp. 2018. All Rights Reserved.
// US Government Users Restricted Rights - Use, duplication or
// disclosure restricted by GSA ADP Schedule Contract with IBM Corp.
// Copyright 2018 IBM Corporation

package crd

import (
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	multierror "github.com/hashicorp/go-multierror"

	istiocrd "istio.io/istio/pilot/pkg/config/kube/crd"
	"istio.io/istio/pilot/pkg/config/memory"
	istiomodel "istio.io/istio/pilot/pkg/model"

	multiclustercrd "github.ibm.com/istio-research/multicluster-roadmap/multicluster/pkg/config/kube/crd"
)

// TODO Merge with version in pkg/agent/config/kube/crd?
// debugClusterInfo simulates the function of K8s Cluster Registry
// https://github.com/kubernetes/cluster-registry in unit tests.
type debugClusterInfo struct {
	ips   map[string]string
	ports map[string]uint32
}

func TestReconcile(t *testing.T) {
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

	tt := []struct {
		added         istiomodel.Config
		deleted       istiomodel.Config
		modified      istiomodel.Config
		istioConfig   []istiomodel.Config
		additions     []istiomodel.Config
		deletions     []istiomodel.Config
		modifications []istiomodel.Config
		wantException bool
	}{
		// Case 0: if we have already configured, adding again won't change things
		{added: loadConfig("rshriram-demo-exposure.yaml", t),
			istioConfig: loadIstioConfigList("rshriram-demo-exposure.yaml", t)},
		// Case 1: If we have nothing configured, adding creates things
		{added: loadConfig("rshriram-demo-exposure.yaml", t),
			additions: loadIstioConfigList("rshriram-demo-exposure.yaml", t)},
		// Case 2: If we delete, the config should go away
		{deleted: loadConfig("rshriram-demo-exposure.yaml", t),
			istioConfig: loadIstioConfigList("rshriram-demo-exposure.yaml", t),
			deletions: []istiomodel.Config{
				istiomodel.Config{
					ConfigMeta: istiomodel.ConfigMeta{
						Type:      "destination-rule",
						Name:      "dest-rule-server-default-notls",
						Namespace: "ns2",
					},
				},
				istiomodel.Config{
					ConfigMeta: istiomodel.ConfigMeta{
						Type:      "gateway",
						Name:      "istio-ingressgateway-server-ns2",
						Namespace: "ns2",
					},
				},
				istiomodel.Config{
					ConfigMeta: istiomodel.ConfigMeta{
						Type:      "virtual-service",
						Name:      "ingressgateway-to-server-ns2",
						Namespace: "ns2",
					},
				},
			},
		},
		// Case 3: If we delete things never realized, we should (internally) fail reconciliation
		{deleted: loadConfig("rshriram-demo-exposure.yaml", t),
			wantException: true},
	}

	for i, tc := range tt {
		t.Run(fmt.Sprintf("Case %d", i), func(t *testing.T) {

			cs, err := createDebugConfigStore(tc.istioConfig)
			if err != nil {
				t.Error(err)
			}

			additions, modifications, errAdditions := AddMulticlusterConfig(cs, tc.added, ci)
			if errAdditions == nil {
				err = checkEqualConfigs(additions, tc.additions)
				if err != nil {
					t.Error(multierror.Prefix(err, "Generated additions unexpected"))
				}
				err = checkEqualConfigs(modifications, tc.modifications)
				if err != nil {
					t.Error(multierror.Prefix(err, "Generated modifications unexpected"))
				}
			}

			modifications, errModifications := ModifyMulticlusterConfig(cs, tc.modified, ci)
			if errModifications == nil {
				err = checkEqualConfigs(modifications, tc.modifications)
				if err != nil {
					t.Error(multierror.Prefix(err, "Generated modifications unexpected"))
				}
			}

			deletions, errDeletions := DeleteMulticlusterConfig(cs, tc.deleted, ci)
			if errDeletions == nil {
				err = checkEqualConfigMetas(deletions, tc.deletions)
				if err != nil {
					t.Error(multierror.Prefix(err, "Proposed deletions unexpected"))
				}
			}

			if tc.wantException {
				if errAdditions == nil && errModifications == nil && errDeletions == nil {
					t.Error("Expected exception; did not receive one")
				}
			} else {
				if errAdditions != nil {
					t.Error(errAdditions)
				}
				if errModifications != nil {
					t.Error(errModifications)
				}
				if errDeletions != nil {
					t.Error(errDeletions)
				}
			}
		})
	}
}

func loadConfig(fname string, t *testing.T) istiomodel.Config {
	configs := loadConfigList(fname, t)

	if len(configs) != 1 {
		t.Fatal(fmt.Errorf("Expected 1 config, got %d", len(configs)))
	}

	return configs[0]
}

// loadConfigList loads *Multicluster* configuration (VirtualService, Gateway, etc) from the test directory
func loadConfigList(fname string, t *testing.T) []istiomodel.Config {
	reader, err := os.Open("../test/expose-binding/" + fname)
	if err != nil {
		t.Fatal(err)
	}

	defer reader.Close() // nolint: errcheck
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	configs, _, err := multiclustercrd.ParseInputs(string(data))
	if err != nil {
		t.Fatal(err)
	}

	return configs
}

// loadIstioConfigList loads *Istio* configuration (VirtualService, Gateway, etc) from the test directory
func loadIstioConfigList(fname string, t *testing.T) []istiomodel.Config {
	reader, err := os.Open("../test/istio-expose-binding/" + fname)
	if err != nil {
		t.Fatal(err)
	}

	defer reader.Close() // nolint: errcheck
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	configs, _, err := istiocrd.ParseInputs(string(data))
	if err != nil {
		t.Fatal(err)
	}

	return configs
}

func createDebugConfigStore(configs []istiomodel.Config) (istiomodel.ConfigStore, error) {
	out := memory.Make(istiomodel.IstioConfigTypes)
	for _, config := range configs {
		_, err := out.Create(config)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func checkEqualConfigs(configs []istiomodel.Config, expected []istiomodel.Config) error {
	if len(configs) != len(expected) {
		return fmt.Errorf("Configurations don't match: different number of elements %d vs expected %d (%#v vs expected %#v)",
			len(configs), len(expected), configs, expected)
	}

	lookup := indexConfigs(expected)
	for _, config := range configs {
		lookupName, ok := lookup[config.Namespace]
		if !ok {
			return fmt.Errorf("Configuration in namespace %q unexpected", config.Namespace)
		}
		lookupType, ok := lookupName[config.Name]
		if !ok {
			return fmt.Errorf("Configuration name %s.%s unexpected", config.Name, config.Namespace)
		}
		expectedConfig, ok := lookupType[config.Type]
		if !ok {
			return fmt.Errorf("Configuration %s.%s type %s unexpected", config.Name, config.Namespace, config.Type)
		}
		// The metadata will be close enough because it was looked up, only compare the Specs
		if !reflect.DeepEqual(config.Spec, expectedConfig.Spec) {
			return fmt.Errorf("Configuration of %s %s.%s %#v unexpected (expected %#v)", config.Type, config.Name, config.Namespace, config.Spec, expectedConfig.Spec)
		}
	}

	// Success, all were found
	return nil
}

// indexConfigs returns a hash of Namespace->Name->Kind = Config
func indexConfigs(configs []istiomodel.Config) map[string]map[string]map[string]istiomodel.Config {

	// Hash the configuration as Namespace->Name->Kind = config
	lookupNS := make(map[string]map[string]map[string]istiomodel.Config)
	for _, config := range configs {
		lookupName, ok := lookupNS[config.Namespace]
		if !ok {
			lookupName = make(map[string]map[string]istiomodel.Config)
			lookupNS[config.Namespace] = lookupName
		}
		lookupType, ok := lookupName[config.Name]
		if !ok {
			lookupType = make(map[string]istiomodel.Config)
			lookupName[config.Name] = lookupType
		}
		lookupType[config.Type] = config
	}

	return lookupNS
}

func checkEqualConfigMetas(configs []istiomodel.Config, expected []istiomodel.Config) error {
	if len(configs) != len(expected) {
		return fmt.Errorf("Configurations don't match: different number of elements %d vs %d (%#v vs %#v)",
			len(configs), len(expected), configs, expected)
	}

	lookup := indexConfigs(expected)
	for _, config := range configs {
		lookupName, ok := lookup[config.Namespace]
		if !ok {
			return fmt.Errorf("Configuration in namespace %q unexpected", config.Namespace)
		}
		lookupType, ok := lookupName[config.Name]
		if !ok {
			return fmt.Errorf("Configuration name %s.%s unexpected", config.Name, config.Namespace)
		}
		_, ok = lookupType[config.Type]
		if !ok {
			return fmt.Errorf("Configuration %s.%s type %s unexpected", config.Name, config.Namespace, config.Type)
		}
		// We only check ConfigMeta and ignore .Spec
	}

	// Success, all were found
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
