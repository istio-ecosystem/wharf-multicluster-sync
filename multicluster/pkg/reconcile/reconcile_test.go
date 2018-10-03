// Licensed Materials - Property of IBM
// (C) Copyright IBM Corp. 2018. All Rights Reserved.
// US Government Users Restricted Rights - Use, duplication or
// disclosure restricted by GSA ADP Schedule Contract with IBM Corp.
// Copyright 2018 IBM Corporation

package reconcile

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	multierror "github.com/hashicorp/go-multierror"

	istiocrd "istio.io/istio/pilot/pkg/config/kube/crd"
	"istio.io/istio/pilot/pkg/config/memory"
	istiomodel "istio.io/istio/pilot/pkg/model"

	multiclustercrd "github.com/istio-ecosystem/wharf-multicluster-sync/multicluster/pkg/config/kube/crd"
	mcmodel "github.com/istio-ecosystem/wharf-multicluster-sync/multicluster/pkg/model"

	kube_v1 "k8s.io/api/core/v1"
)

// TODO Merge with version in pkg/agent/config/kube/crd?
// debugClusterInfo simulates the function of K8s Cluster Registry
// https://github.com/kubernetes/cluster-registry in unit tests.
type debugClusterInfo struct {
	ips   map[string]string
	ports map[string]uint32
}

func TestReconcileBinding(t *testing.T) {
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
		added            *istiomodel.Config
		deleted          *istiomodel.Config
		modified         *istiomodel.Config
		istioConfig      []istiomodel.Config
		initialServices  []kube_v1.Service
		additions        []istiomodel.Config
		deletions        []istiomodel.Config
		modifications    []istiomodel.Config
		svcAdditions     []kube_v1.Service
		svcModifications []kube_v1.Service
		// TODO svcDeletions []kube_v1.Service
		wantException bool
		style         string
	}{
		// Case 0: if we have already configured, adding again won't change things
		{added: loadConfig("reviews-binding.yaml", t),
			istioConfig: loadIstioConfigList("reviews-directingress-binding-nonamespace.yaml.golden", t),
			initialServices: loadK8sServiceListFrom("reviews-directingress-binding-starter.yaml",
				"../test/expose-binding/", t),
			style: mcmodel.DirectIngressStyle},
		// Case 1: If we have nothing configured, adding creates things
		{added: loadConfig("reviews-binding.yaml", t),
			additions:    loadIstioConfigList("reviews-directingress-binding-nonamespace.yaml.golden", t),
			svcAdditions: loadK8sServiceList("reviews-directingress-binding.yaml.golden", t),
			style:        mcmodel.DirectIngressStyle},
	}

	for i, tc := range tt {
		t.Run(fmt.Sprintf("Case %d", i), func(t *testing.T) {
			if tc.style != "" {
				os.Setenv(mcmodel.IstioConversionStyleKey, tc.style) // nolint: errcheck
			} else {
				os.Setenv(mcmodel.IstioConversionStyleKey, mcmodel.EgressIngressStyle) // nolint: errcheck
			}

			cs, err := createDebugConfigStore(tc.istioConfig)
			if err != nil {
				t.Error(err)
			}

			r := NewReconciler(cs, tc.initialServices, ci)
			var errAdditions error
			var errModifications error
			var errDeletions error
			if tc.added != nil {
				var addChanges *ConfigChanges
				addChanges, errAdditions = r.AddMulticlusterConfig(*tc.added)
				if errAdditions == nil {
					err = checkEqualConfigs(addChanges.Additions, tc.additions)
					if err != nil {
						t.Error(multierror.Prefix(err, "Generated additions unexpected:"))
					}
					err = checkEqualConfigs(addChanges.Modifications, tc.modifications)
					if err != nil {
						t.Error(multierror.Prefix(err, "Generated modifications unexpected:"))
					}
					err = checkEqualServices(addChanges.Kubernetes.Additions, tc.svcAdditions)
					if err != nil {
						t.Error(multierror.Prefix(err, "Generated K8s modifications unexpected:"))
					}
					err = checkEqualServices(addChanges.Kubernetes.Modifications, tc.svcModifications)
					if err != nil {
						t.Error(multierror.Prefix(err, "Generated K8s modifications unexpected:"))
					}
				}
			}

			if tc.modified != nil {
				var modChanges *ConfigChanges
				modChanges, errModifications = r.ModifyMulticlusterConfig(*tc.modified)
				if errModifications == nil {
					err = checkEqualConfigs(modChanges.Modifications, tc.modifications)
					if err != nil {
						t.Error(multierror.Prefix(err, "Generated modifications unexpected"))
					}
					err = checkEqualServices(modChanges.Kubernetes.Modifications, tc.svcModifications)
					if err != nil {
						t.Error(multierror.Prefix(err, "Generated modifications unexpected"))
					}
				}
			}

			if tc.deleted != nil {
				var delChanges *ConfigChanges
				delChanges, errDeletions = r.DeleteMulticlusterConfig(*tc.deleted)
				if errDeletions == nil {
					err = checkEqualConfigMetas(delChanges.Deletions, tc.deletions)
					if err != nil {
						t.Error(multierror.Prefix(err, "Proposed deletions unexpected"))
					}
					// TODO Check deletions
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

func TestReconcileExposure(t *testing.T) {
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
		added            *istiomodel.Config
		deleted          *istiomodel.Config
		modified         *istiomodel.Config
		istioConfig      []istiomodel.Config
		initialServices  []kube_v1.Service
		additions        []istiomodel.Config
		deletions        []istiomodel.Config
		modifications    []istiomodel.Config
		svcAdditions     []kube_v1.Service
		svcModifications []kube_v1.Service
		// TODO svcDeletions []kube_v1.Service
		wantException bool
		style         string
	}{
		// Case 0: if we have already configured, adding again won't change things
		{added: loadConfig("rshriram-demo-exposure.yaml", t),
			istioConfig: loadIstioConfigList("rshriram-demo-exposure.yaml.golden", t)},
		// Case 1: If we have nothing configured, adding creates things
		{added: loadConfig("rshriram-demo-exposure.yaml", t),
			additions: loadIstioConfigList("rshriram-demo-exposure.yaml.golden", t)},
		// Case 2: If we delete, the config should go away
		{deleted: loadConfig("rshriram-demo-exposure.yaml", t),
			istioConfig: loadIstioConfigList("rshriram-demo-exposure.yaml.golden", t),
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
		// Case 4: Direct Ingress style
		{added: loadConfig("rshriram-demo-exposure.yaml", t),
			istioConfig: loadIstioConfigList("banix-demo-exposure.yaml.golden", t),
			style:       mcmodel.DirectIngressStyle},
		// Case 5: Direct Ingress style with subset
		{added: loadConfig("reviews-exposure-v1-only.yaml", t),
			istioConfig:   loadIstioConfigListFrom("reviews-exposure-starter.yaml", "../test/expose-binding/", t),
			additions:     loadIstioConfigList("reviews-sni-exposure-v1-only-additions.yaml.golden", t),
			modifications: loadIstioConfigList("reviews-sni-exposure-v1-only-modifications.yaml.golden", t),
			style:         mcmodel.DirectIngressStyle},
		// Case 6: Direct Ingress style, no subset, DR already exists
		{added: loadConfig("reviews-exposure.yaml", t),
			istioConfig:   loadIstioConfigListFrom("reviews-exposure-starter.yaml", "../test/expose-binding/", t),
			additions:     loadIstioConfigList("reviews-sni-exposure-additions.yaml.golden", t),
			modifications: loadIstioConfigList("reviews-sni-exposure-modifications.yaml.golden", t),
			style:         mcmodel.DirectIngressStyle},
	}

	for i, tc := range tt {
		t.Run(fmt.Sprintf("Case %d", i), func(t *testing.T) {
			if tc.style != "" {
				os.Setenv(mcmodel.IstioConversionStyleKey, tc.style) // nolint: errcheck
			} else {
				os.Setenv(mcmodel.IstioConversionStyleKey, mcmodel.EgressIngressStyle) // nolint: errcheck
			}

			cs, err := createDebugConfigStore(tc.istioConfig)
			if err != nil {
				t.Error(err)
			}

			r := NewReconciler(cs, tc.initialServices, ci)
			var errAdditions error
			var errModifications error
			var errDeletions error
			if tc.added != nil {
				var addChanges *ConfigChanges
				addChanges, errAdditions = r.AddMulticlusterConfig(*tc.added)
				if errAdditions == nil {
					err = checkEqualConfigs(addChanges.Additions, tc.additions)
					if err != nil {
						t.Error(multierror.Prefix(err, "Generated additions unexpected"))
					}
					err = checkEqualConfigs(addChanges.Modifications, tc.modifications)
					if err != nil {
						t.Error(multierror.Prefix(err, "Generated modifications unexpected"))
					}
					err = checkEqualServices(addChanges.Kubernetes.Additions, tc.svcAdditions)
					if err != nil {
						t.Error(multierror.Prefix(err, "Generated modifications unexpected"))
					}
					err = checkEqualServices(addChanges.Kubernetes.Modifications, tc.svcModifications)
					if err != nil {
						t.Error(multierror.Prefix(err, "Generated modifications unexpected"))
					}
				}
			}

			if tc.modified != nil {
				var modChanges *ConfigChanges
				modChanges, errModifications = r.ModifyMulticlusterConfig(*tc.modified)
				if errModifications == nil {
					err = checkEqualConfigs(modChanges.Modifications, tc.modifications)
					if err != nil {
						t.Error(multierror.Prefix(err, "Generated modifications unexpected"))
					}
					err = checkEqualServices(modChanges.Kubernetes.Modifications, tc.svcModifications)
					if err != nil {
						t.Error(multierror.Prefix(err, "Generated modifications unexpected"))
					}
				}
			}

			if tc.deleted != nil {
				var delChanges *ConfigChanges
				delChanges, errDeletions = r.DeleteMulticlusterConfig(*tc.deleted)
				if errDeletions == nil {
					err = checkEqualConfigMetas(delChanges.Deletions, tc.deletions)
					if err != nil {
						t.Error(multierror.Prefix(err, "Proposed deletions unexpected"))
					}
					// TODO Check deletions
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

func loadConfig(fname string, t *testing.T) *istiomodel.Config {
	configs := loadConfigList(fname, t)

	if len(configs) != 1 {
		t.Fatal(fmt.Errorf("expected 1 config, got %d", len(configs)))
	}

	return &configs[0]
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

// loadK8sServiceList loads *Kubernetes* configuration (currently just Services) from the test directory
func loadK8sServiceList(fname string, t *testing.T) []kube_v1.Service {
	return loadK8sServiceListFrom(fname, "../test/istio-expose-binding/", t)
}

// loadK8sServiceListFrom loads *Kubernetes* configuration (currently just Services) from the test directory
func loadK8sServiceListFrom(fname string, dirname string, t *testing.T) []kube_v1.Service {
	reader, err := os.Open(dirname + fname)
	if err != nil {
		t.Fatal(err)
	}

	defer reader.Close() // nolint: errcheck
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	outSvcs := make([]kube_v1.Service, 0)
	_, kinds, err := istiocrd.ParseInputs(string(data))
	if err != nil {
		t.Fatal(err)
	}

	for _, nonIstio := range kinds {
		if nonIstio.Kind == "Service" &&
			nonIstio.APIVersion == "v1" {

			svc, err := parseService(nonIstio)
			if err != nil {
				t.Fatal(err)
			}

			outSvcs = append(outSvcs, *svc)
		}
	}

	return outSvcs
}

func parseService(unparsed istiocrd.IstioKind) (*kube_v1.Service, error) {
	// To convert unparsed to a v1beta1.Ingress Marshal into JSON and Unmarshal back
	b, err := json.Marshal(unparsed)
	if err != nil {
		return nil, multierror.Prefix(err, "can't reserialize Service")
	}

	out := &kube_v1.Service{}
	err = json.Unmarshal(b, out)
	if err != nil {
		return nil, multierror.Prefix(err, "can't deserialize as Service")
	}

	return out, nil
}

// loadIstioConfigList loads *Istio* configuration (VirtualService, Gateway, etc) from the test directory
func loadIstioConfigList(fname string, t *testing.T) []istiomodel.Config {
	return loadIstioConfigListFrom(fname, "../test/istio-expose-binding/", t)
}

// loadIstioConfigListFrom loads *Istio* configuration (VirtualService, Gateway, etc) from the test directory
func loadIstioConfigListFrom(fname string, dirname string, t *testing.T) []istiomodel.Config {
	reader, err := os.Open(dirname + fname)
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
		return fmt.Errorf("configurations don't match: different number of elements %d vs expected %d (%#v vs expected %#v)",
			len(configs), len(expected), configs, expected)
	}

	lookup := indexConfigs(expected)
	for _, config := range configs {
		lookupName, ok := lookup[config.Namespace]
		if !ok {
			return fmt.Errorf("configuration in namespace %q unexpected", config.Namespace)
		}
		lookupType, ok := lookupName[config.Name]
		if !ok {
			return fmt.Errorf("configuration name %s.%s unexpected", config.Name, config.Namespace)
		}
		expectedConfig, ok := lookupType[config.Type]
		if !ok {
			return fmt.Errorf("configuration %s.%s type %s unexpected", config.Name, config.Namespace, config.Type)
		}
		// The metadata will be close enough because it was looked up, only compare the Specs
		if !reflect.DeepEqual(config.Spec, expectedConfig.Spec) {
			wanted, _ := json.Marshal(expectedConfig.Spec)
			got, _ := json.Marshal(config.Spec)
			return fmt.Errorf("configuration of %s %s.%s %s unexpected (expected %s)", config.Type, config.Name, config.Namespace, string(got), string(wanted))
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
		return fmt.Errorf("configurations don't match: different number of elements %d vs %d (%#v vs %#v)",
			len(configs), len(expected), configs, expected)
	}

	lookup := indexConfigs(expected)
	for _, config := range configs {
		lookupName, ok := lookup[config.Namespace]
		if !ok {
			return fmt.Errorf("configuration in namespace %q unexpected", config.Namespace)
		}
		lookupType, ok := lookupName[config.Name]
		if !ok {
			return fmt.Errorf("configuration name %s.%s unexpected", config.Name, config.Namespace)
		}
		_, ok = lookupType[config.Type]
		if !ok {
			return fmt.Errorf("configuration %s.%s type %s unexpected", config.Name, config.Namespace, config.Type)
		}
		// We only check ConfigMeta and ignore .Spec
	}

	// Success, all were found
	return nil
}

func (ci debugClusterInfo) IP(name string) string {
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

func checkEqualServices(svcs []kube_v1.Service, expected []kube_v1.Service) error {
	if len(svcs) != len(expected) {
		return fmt.Errorf("service definitions don't match: different number of elements %d vs expected %d (%#v vs expected %#v)",
			len(svcs), len(expected), svcs, expected)
	}

	lookup := indexServices(expected, svcIndex)
	for _, svc := range svcs {
		expectedSvc, ok := lookup[svcIndex(svc)]
		if !ok {
			return fmt.Errorf("service %s.%s (kind %s) unexpected", svc.Name, svc.Namespace, svc.Kind)
		}
		// The metadata will be close enough because it was looked up, only compare the Specs
		if !reflect.DeepEqual(svc.Spec, expectedSvc.Spec) {
			wanted, _ := json.Marshal(expectedSvc.Spec)
			got, _ := json.Marshal(svc.Spec)
			return fmt.Errorf("configuration of %s %s.%s %s unexpected (expected %s)", svc.Kind, svc.Name, svc.Namespace, string(got), string(wanted))
		}
	}

	// Success, all were found
	return nil
}
