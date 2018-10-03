// Licensed Materials - Property of IBM
// (C) Copyright IBM Corp. 2018. All Rights Reserved.
// US Government Users Restricted Rights - Use, duplication or
// disclosure restricted by GSA ADP Schedule Contract with IBM Corp.
// Copyright 2018 IBM Corporation

package model

import (
	"fmt"
	"sort"

	"github.com/istio-ecosystem/wharf-multicluster-sync/api/multicluster/v1alpha1"

	"istio.io/api/networking/v1alpha3"
	istiomodel "istio.io/istio/pilot/pkg/model"

	kube_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	multierror "github.com/hashicorp/go-multierror"
)

// ConvertBindingsAndExposuresDirectIngress converts a list of multicluster SEP and RDS configuration
// into Istio configuration.  It may consult existing Istio configuration in 'store' (e.g. DestinationRule subsets)
func ConvertBindingsAndExposuresDirectIngress(mcs []istiomodel.Config, ci ClusterInfo, store istiomodel.ConfigStore, svcs []kube_v1.Service) ([]istiomodel.Config, []kube_v1.Service, error) { // nolint: lll
	out := make([]istiomodel.Config, 0)
	outServices := make([]kube_v1.Service, 0)

	// Construct map of hostname -> DestinationRule (needed for merging for subsets)
	drs, err := mapHostnameToDestinationRule(store)
	if err != nil {
		return nil, nil, err
	}

	// Construct map of hostname -> ServiceEntry (needed for merging for multiple endpoints)
	vss, err := mapHostnameToServiceEntry(store)
	if err != nil {
		return nil, nil, err
	}

	// Process each Multicluster Config SEP or RSB
	for _, mc := range mcs {
		var istio []istiomodel.Config
		var svcs []kube_v1.Service
		var err error
		rsb, ok := mc.Spec.(*v1alpha1.RemoteServiceBinding)
		if ok {
			istio, svcs, err = convertRSBDirectIngress(mc, rsb, vss, ci)
		}
		sep, ok := mc.Spec.(*v1alpha1.ServiceExpositionPolicy)
		if ok {
			istio, err = convertSEPDirectIngress(mc, sep, drs)
		}
		if err != nil {
			return out, outServices, multierror.Prefix(err, "Could not convert")
		}
		out = append(out, istio...)
		outServices = append(outServices, svcs...)
	}

	return uniquifyIstio(out), uniquifyServices(outServices), nil
}

// Construct map of hostname -> DestinationRule
func mapHostnameToDestinationRule(store istiomodel.ConfigStore) (map[string]*istiomodel.Config, error) {
	drs := make(map[string]*istiomodel.Config)
	drConfigs, err := store.List(istiomodel.DestinationRule.Type, meta_v1.NamespaceDefault)
	if err != nil {
		return nil, err
	}
	for _, dr := range drConfigs {
		spec := dr.Spec.(*v1alpha3.DestinationRule)

		// Deep copy some of DR so we don't modify the original while merging for subsets
		newDR := dr
		newSpec := *spec
		newSpec.Subsets = make([]*v1alpha3.Subset, len(newSpec.Subsets))
		copy(newSpec.Subsets, spec.Subsets)
		newDR.Spec = &newSpec

		drs[spec.Host] = &newDR
	}
	return drs, nil
}

// Construct map of hostname -> ServiceEntry
func mapHostnameToServiceEntry(store istiomodel.ConfigStore) (map[string]*istiomodel.Config, error) {
	serviceEntries := make(map[string]*istiomodel.Config)
	seConfigs, err := store.List(istiomodel.ServiceEntry.Type, meta_v1.NamespaceDefault)
	if err != nil {
		return nil, err
	}
	for _, se := range seConfigs {
		spec := se.Spec.(*v1alpha3.ServiceEntry)

		// Deep copy Endpoint of SE so we don't modify the original while merging for addresses
		newSE := se
		newSpec := *spec
		newSpec.Endpoints = make([]*v1alpha3.ServiceEntry_Endpoint, 0)
		for _, endpoint := range spec.Endpoints {
			newEndpoint := &v1alpha3.ServiceEntry_Endpoint{
				Address: endpoint.Address,
				Ports:   make(map[string]uint32),
			}
			for protocol, port := range endpoint.Ports {
				newEndpoint.Ports[protocol] = port
			}
			newSpec.Endpoints = append(newSpec.Endpoints, newEndpoint)
		}
		newSE.Spec = &newSpec

		for _, host := range spec.Hosts {
			serviceEntries[host] = &newSE
		}
	}
	return serviceEntries, nil
}

// uniqueifyIstio removes duplicates (e.g. DRs for the same host exposed under different aliases),
// favoring duplicates later in the sequence.
func uniquifyIstio(configs []istiomodel.Config) []istiomodel.Config {
	unique := make([]istiomodel.Config, 0)
	names := make(map[string]int) // map of type+namespace+name -> position
	for _, config := range configs {
		key := fmt.Sprintf("%s+%s+%s", config.Type, config.Namespace, config.Name)
		pos, ok := names[key]
		if ok {
			unique[pos] = config
		} else {
			names[key] = len(unique)
			unique = append(unique, config)
		}
	}

	return unique
}

// uniqueifyServices
func uniquifyServices(configs []kube_v1.Service) []kube_v1.Service {
	unique := make([]kube_v1.Service, 0)
	names := make(map[string]int) // map of namespace+name -> position
	for _, config := range configs {
		key := fmt.Sprintf("%s+%s", config.Namespace, config.Name)
		pos, ok := names[key]
		if ok {
			unique[pos] = config
		} else {
			names[key] = len(unique)
			unique = append(unique, config)
		}
	}

	return unique
}

func convertRSBDirectIngress(config istiomodel.Config, rsb *v1alpha1.RemoteServiceBinding,
	serviceEntries map[string]*istiomodel.Config, ci ClusterInfo) ([]istiomodel.Config,
	[]kube_v1.Service, error) {
	out := make([]istiomodel.Config, 0)
	outSvcs := make([]kube_v1.Service, 0)

	for _, remote := range rsb.Remote {
		for _, svc := range remote.Services {
			out = append(out, *serviceToServiceEntryDirectIngress(svc, config,
				serviceEntries, ci.IP(remote.Cluster), ci.Port(remote.Cluster)))
			out = append(out, *serviceToDestinationRuleDirectIngress(svc, config))
			outSvcs = append(outSvcs, *serviceToKubernetesServiceDirectIngress(svc, config))
		}
	}

	return out, outSvcs, nil
}

// serviceToServiceEntry() creates a ServiceEntry pointing to istio-egressgateway
func serviceToServiceEntryDirectIngress(rs *v1alpha1.RemoteServiceBinding_RemoteCluster_RemoteService, config istiomodel.Config, serviceEntries map[string]*istiomodel.Config, ip string, port uint32) *istiomodel.Config { // nolint: lll
	hostname := rsHostname(rs)
	protocol := "http"
	serviceEntry, ok := serviceEntries[hostname]
	if !ok {
		serviceEntry = &istiomodel.Config{
			ConfigMeta: istiomodel.ConfigMeta{
				Type:        istiomodel.ServiceEntry.Type,
				Group:       istiomodel.ServiceEntry.Group + istiomodel.IstioAPIGroupDomain,
				Version:     istiomodel.ServiceEntry.Version,
				Name:        fmt.Sprintf("service-entry-%s", remoteServiceName(rs)),
				Namespace:   config.Namespace,
				Annotations: annotations(config),
			},
			Spec: &v1alpha3.ServiceEntry{
				Hosts: []string{rsHostname(rs)},
				Ports: []*v1alpha3.Port{
					&v1alpha3.Port{
						Number:   portClientUses(rs),
						Protocol: "HTTP",
						Name:     "http",
					},
				},
				Location:   v1alpha3.ServiceEntry_MESH_EXTERNAL,
				Resolution: v1alpha3.ServiceEntry_STATIC,
				Endpoints:  []*v1alpha3.ServiceEntry_Endpoint{},
			},
		}

		serviceEntries[hostname] = serviceEntry
	}

	// Ensure serviceEntry has an endpoint for IP
	spec := serviceEntry.Spec.(*v1alpha3.ServiceEntry)
	endpoint := getEndpoint(spec, ip)
	if endpoint == nil {
		endpoint = &v1alpha3.ServiceEntry_Endpoint{
			Address: ip,
			Ports:   make(map[string]uint32),
		}
		spec.Endpoints = append(spec.Endpoints, endpoint)
	}

	// Ensure serviceEntry.Endpoint has a Port for protocol
	// TODO Check that the port matches and return error otherwise (unmergable)
	_, ok = endpoint.Ports[protocol]
	if !ok {
		endpoint.Ports[protocol] = port
	}

	// Ensure the endpoints are sorted (not needed for Istio, needed for go tests)
	sort.Slice(spec.Endpoints, func(i, j int) bool {
		return spec.Endpoints[i].Address < spec.Endpoints[j].Address
		// TODO Ensure the endpoints are stable (e.g. sorted) when there are multiple matching IPs with different protocols/ports
		// Istio doesn't need them sorted, but the tests expect stable generation
	})

	return serviceEntry
}

func getEndpoint(serviceEntry *v1alpha3.ServiceEntry, ip string) *v1alpha3.ServiceEntry_Endpoint {
	for _, endpoint := range serviceEntry.Endpoints {
		if endpoint.Address == ip {
			return endpoint
		}
	}

	return nil
}

// portClientUses yields the TCP port that clients expect to invoke
func portClientUses(rs *v1alpha1.RemoteServiceBinding_RemoteCluster_RemoteService) uint32 {
	if rs.Port == 0 {
		return 80
	}
	return rs.Port
}

// serviceToDestinationRuleDirectIngress() creates a DestinationRule setting up MUTUAL (not ISTIO_MUTUAL) TLS
func serviceToDestinationRuleDirectIngress(rs *v1alpha1.RemoteServiceBinding_RemoteCluster_RemoteService, config istiomodel.Config) *istiomodel.Config {
	return &istiomodel.Config{
		ConfigMeta: istiomodel.ConfigMeta{
			Type:        istiomodel.DestinationRule.Type,
			Group:       istiomodel.DestinationRule.Group + istiomodel.IstioAPIGroupDomain,
			Version:     istiomodel.DestinationRule.Version,
			Name:        fmt.Sprintf("dest-rule-%s", remoteServiceName(rs)),
			Namespace:   config.Namespace,
			Annotations: annotations(config),
		},
		Spec: &v1alpha3.DestinationRule{
			Host: rsHostname(rs),
			TrafficPolicy: &v1alpha3.TrafficPolicy{
				Tls: &v1alpha3.TLSSettings{
					Mode:              v1alpha3.TLSSettings_MUTUAL,
					ClientCertificate: "/etc/certs/cert-chain.pem",
					PrivateKey:        "/etc/certs/key.pem",
					CaCertificates:    "/etc/certs/root-cert.pem",
					Sni:               rsAliasHostname(rs),
				},
			},
		},
	}
}

// serviceToKubernetesServiceDirectIngress() creates a K8s Service so that DNS resolves to something/anything
func serviceToKubernetesServiceDirectIngress(rs *v1alpha1.RemoteServiceBinding_RemoteCluster_RemoteService, config istiomodel.Config) *kube_v1.Service {
	return &kube_v1.Service{
		TypeMeta: meta_v1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: meta_v1.ObjectMeta{
			Name:        remoteServiceName(rs),
			Namespace:   config.Namespace,
			Annotations: annotations(config),
		},
		Spec: kube_v1.ServiceSpec{
			Type: kube_v1.ServiceTypeClusterIP,
			Ports: []kube_v1.ServicePort{
				kube_v1.ServicePort{
					Protocol: "TCP",
					Port:     int32(portClientUses(rs)),
				},
			},
			// No selector
			// ClusterIP will be assigned by the master
		},
	}
}

func rsAliasHostname(rs *v1alpha1.RemoteServiceBinding_RemoteCluster_RemoteService) string {
	// We give a .local rather than .global hostname so that we can use a K8s Service
	// to create the DNS and keep apps from knowing the communication is multi-cluster
	return fmt.Sprintf("%s.%s.svc.cluster.local", rs.Name, remoteServiceNamespace(rs))
}

func convertSEPDirectIngress(config istiomodel.Config, sep *v1alpha1.ServiceExpositionPolicy, drs map[string]*istiomodel.Config) ([]istiomodel.Config, error) {
	out := make([]istiomodel.Config, 0)

	for _, remote := range sep.Exposed {
		dr, err := expositionToDestinationRuleDirectIngress(remote, config, drs)
		if err != nil {
			return out, err
		}

		gw, err := expositionToGatewayDirectIngress(remote, config)
		if err != nil {
			return out, err
		}

		vs, err := expositionToVirtualServiceDirectIngress(remote, config)
		if err != nil {
			return out, err
		}

		out = append(out, *dr, *gw, *vs)
	}

	return out, nil
}

// 'drs' maps hostname to DestinationRule and is used to keep track of destinations exposed with different subset and/or alias
func expositionToDestinationRuleDirectIngress(es *v1alpha1.ServiceExpositionPolicy_ExposedService,
	config istiomodel.Config, drs map[string]*istiomodel.Config) (*istiomodel.Config, error) {
	hostname := fmt.Sprintf("%s.default.svc.cluster.local", es.Name)

	dr, ok := drs[hostname]
	if !ok {
		dr = &istiomodel.Config{
			ConfigMeta: istiomodel.ConfigMeta{
				Type:    istiomodel.DestinationRule.Type,
				Group:   istiomodel.DestinationRule.Group + istiomodel.IstioAPIGroupDomain,
				Version: istiomodel.DestinationRule.Version,
				Name:    fmt.Sprintf("dest-rule-%s-default-notls", es.Name), // TODO avoid collisions?
				// Namespace:   config.Namespace,
				Annotations: annotations(config),
			},
			Spec: &v1alpha3.DestinationRule{
				Host:    hostname,
				Subsets: []*v1alpha3.Subset{},
			},
		}

		drs[hostname] = dr
	}

	// Ensure dr has a subset named 'notls' or 'notls-<orig>' for the subset
	spec := dr.Spec.(*v1alpha3.DestinationRule)
	subset := getSubset(spec, notlsSubsetName(es))
	if subset == nil {
		var labels map[string]string
		if es.Subset != "" {
			origSubset := getSubset(spec, es.Subset)
			if origSubset == nil {
				// TODO should we fail or generate a default?
				// return nil, fmt.Errorf("Exposed subset %q not defined in DestinationRule %s/%s", es.Subset, dr.Namespace, dr.Name)
				labels = nil
			} else {
				labels = origSubset.Labels
			}
		} else {
			labels = nil
		}

		spec.Subsets = append(spec.Subsets, &v1alpha3.Subset{
			Name: notlsSubsetName(es),
			TrafficPolicy: &v1alpha3.TrafficPolicy{
				Tls: &v1alpha3.TLSSettings{
					Mode: v1alpha3.TLSSettings_DISABLE,
				},
			},
			Labels: labels,
		})
	}

	// Ensure the subsets are sorted (not needed for Istio, needed for go tests)
	sort.Slice(spec.Subsets, func(i, j int) bool {
		return spec.Subsets[i].Name < spec.Subsets[j].Name
	})

	return dr, nil
}

// notlsSubsetName returns the name of the Subset to be used for Istio configuration
func notlsSubsetName(es *v1alpha1.ServiceExpositionPolicy_ExposedService) string {
	if es.Subset != "" {
		return fmt.Sprintf("notls-%s", es.Subset)
	}
	return "notls"
}

func getSubset(rule *v1alpha3.DestinationRule, name string) *v1alpha3.Subset {
	for _, subset := range rule.Subsets {
		if subset.Name == name {
			return subset
		}
	}

	return nil
}

func expositionToGatewayDirectIngress(es *v1alpha1.ServiceExpositionPolicy_ExposedService, config istiomodel.Config) (*istiomodel.Config, error) {
	return &istiomodel.Config{
		ConfigMeta: istiomodel.ConfigMeta{
			Type:        istiomodel.Gateway.Type,
			Group:       istiomodel.Gateway.Group + istiomodel.IstioAPIGroupDomain,
			Version:     istiomodel.Gateway.Version,
			Name:        exposedServiceGatewayName(es, config),
			Namespace:   meta_v1.NamespaceDefault, // TODO config.Namespace?
			Annotations: annotations(config),
		},
		Spec: &v1alpha3.Gateway{
			Servers: []*v1alpha3.Server{
				&v1alpha3.Server{
					Port: &v1alpha3.Port{
						Number:   80,
						Protocol: "TLS",
						Name:     fmt.Sprintf("%s-%s-%d", es.Name, getNamespace(config), 80),
					},
					// We give a .local rather than .global hostname so that we can use a K8s Service
					// to create the DNS and keep apps from knowing the communication is multi-cluster
					Hosts: []string{fmt.Sprintf("%s.%s.svc.cluster.local", exposedServiceName(es), getNamespace(config))},
					Tls: &v1alpha3.Server_TLSOptions{
						Mode: v1alpha3.Server_TLSOptions_PASSTHROUGH,
					},
				},
			},
			Selector: map[string]string{"istio": "ingressgateway"},
		},
	}, nil
}

// expositionToVirtualServiceDirectIngress() creates a VirtualService with sniHosts
func expositionToVirtualServiceDirectIngress(es *v1alpha1.ServiceExpositionPolicy_ExposedService, config istiomodel.Config) (*istiomodel.Config, error) {
	return &istiomodel.Config{
		ConfigMeta: istiomodel.ConfigMeta{
			Type:        istiomodel.VirtualService.Type,
			Group:       istiomodel.VirtualService.Group + istiomodel.IstioAPIGroupDomain,
			Version:     istiomodel.VirtualService.Version,
			Name:        fmt.Sprintf("ingressgateway-to-%s-%s", exposedServiceName(es), getNamespace(config)),
			Namespace:   meta_v1.NamespaceDefault, // TODO config.Namespace?
			Annotations: annotations(config),
		},
		Spec: &v1alpha3.VirtualService{
			Hosts:    []string{esHostname(config, es)},
			Gateways: []string{exposedServiceGatewayName(es, config)},
			Tls: []*v1alpha3.TLSRoute{
				&v1alpha3.TLSRoute{
					Match: []*v1alpha3.TLSMatchAttributes{
						&v1alpha3.TLSMatchAttributes{
							SniHosts: []string{esHostname(config, es)},
							Port:     80,
						},
					},
					Route: []*v1alpha3.DestinationWeight{
						&v1alpha3.DestinationWeight{
							Destination: &v1alpha3.Destination{
								Host:   fmt.Sprintf("%s.%s.svc.cluster.local", es.Name, meta_v1.NamespaceDefault),
								Subset: notlsSubsetName(es),
								Port: &v1alpha3.PortSelector{
									Port: &v1alpha3.PortSelector_Number{
										Number: portServiceExposes(es),
									},
								},
							},
						},
					},
				},
			},
		},
	}, nil
}

// portServiceExposes yields the TCP port the K8s service listens on
func portServiceExposes(es *v1alpha1.ServiceExpositionPolicy_ExposedService) uint32 {
	if es.Port == 0 {
		return 80
	}
	return es.Port
}
