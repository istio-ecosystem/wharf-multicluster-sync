// Licensed Materials - Property of IBM
// (C) Copyright IBM Corp. 2018. All Rights Reserved.
// US Government Users Restricted Rights - Use, duplication or
// disclosure restricted by GSA ADP Schedule Contract with IBM Corp.
// Copyright 2018 IBM Corporation

package model

import (
	"fmt"

	"github.ibm.com/istio-research/multicluster-roadmap/api/multicluster/v1alpha1"

	"istio.io/api/networking/v1alpha3"
	istiomodel "istio.io/istio/pilot/pkg/model"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	multierror "github.com/hashicorp/go-multierror"
)

// ConvertBindingsAndExposuresSNI converts a list of multicluster SEP and RDS configuration
// into Istio configuration.  It may consult existing Istio configuration in 'store' (e.g. DestinationRule subsets)
func ConvertBindingsAndExposuresDirectIngress(mcs []istiomodel.Config, ci ClusterInfo, store istiomodel.ConfigStore) ([]istiomodel.Config, error) {
	out := make([]istiomodel.Config, 0)

	// Construct map of hostname -> DestinationRule
	drs := make(map[string]*istiomodel.Config)
	drConfigs, err := store.List(istiomodel.DestinationRule.Type, meta_v1.NamespaceDefault)
	if err != nil {
		return nil, err
	}
	for _, dr := range drConfigs {
		spec := dr.Spec.(*v1alpha3.DestinationRule)
		dr.ResourceVersion = "" // Don't tie rule to a specific version
		drs[spec.Host] = &dr
	}

	// Process each Multicluster Config SEP or RSB
	for _, mc := range mcs {
		var istio []istiomodel.Config
		var err error
		rsb, ok := mc.Spec.(*v1alpha1.RemoteServiceBinding)
		if ok {
			istio, err = convertRSBSNI(mc, rsb, ci)
		}
		sep, ok := mc.Spec.(*v1alpha1.ServiceExpositionPolicy)
		if ok {
			istio, err = convertSEPSNI(mc, sep, drs)
		}
		if err != nil {
			return out, multierror.Prefix(err, "Could not convert")
		}
		out = append(out, istio...)
	}

	// Remove duplicates (e.g. DRs for the same host exposed under different aliases),
	// favoring duplicates later in the sequence.
	unique := make([]istiomodel.Config, 0)
	names := make(map[string]int) // map of type+namespace+name -> position
	for _, config := range out {
		key := fmt.Sprintf("%s+%s+%s", config.Type, config.Namespace, config.Name)
		pos, ok := names[key]
		if ok {
			unique[pos] = config
		} else {
			names[key] = len(unique)
			unique = append(unique, config)
		}
	}

	return unique, nil
}

func convertRSBSNI(config istiomodel.Config, rsb *v1alpha1.RemoteServiceBinding, ci ClusterInfo) ([]istiomodel.Config, error) {
	out := make([]istiomodel.Config, 0)

	for _, remote := range rsb.Remote {
		for _, svc := range remote.Services {
			out = append(out, *serviceToServiceEntrySNI(svc, config, ci.Ip(remote.Cluster), ci.Port(remote.Cluster)))
			out = append(out, *serviceToDestinationRuleSNI(svc, config))
		}
	}

	return out, nil
}

// serviceToServiceEntry() creates a ServiceEntry pointing to istio-egressgateway
func serviceToServiceEntrySNI(rs *v1alpha1.RemoteServiceBinding_RemoteCluster_RemoteService, config istiomodel.Config, ip string, port uint32) *istiomodel.Config {
	return &istiomodel.Config{
		ConfigMeta: istiomodel.ConfigMeta{
			Type:        istiomodel.ServiceEntry.Type,
			Group:       istiomodel.ServiceEntry.Group + istiomodel.IstioAPIGroupDomain,
			Version:     istiomodel.ServiceEntry.Version,
			Name:        fmt.Sprintf("service-entry-%s", config.Name), // TODO avoid collisions?
			Namespace:   config.Namespace,
			Annotations: annotations(config),
		},
		Spec: &v1alpha3.ServiceEntry{
			Hosts: []string{rsHostname(rs)},
			Ports: []*v1alpha3.Port{
				&v1alpha3.Port{
					Number:   80,
					Protocol: "HTTP",
					Name:     "http",
				},
			},
			Location:   v1alpha3.ServiceEntry_MESH_EXTERNAL,
			Resolution: v1alpha3.ServiceEntry_STATIC,
			Endpoints: []*v1alpha3.ServiceEntry_Endpoint{
				&v1alpha3.ServiceEntry_Endpoint{
					Address: ip,
					Ports:   map[string]uint32{"http": port},
				},
			},
		},
	}
}

// serviceToDestinationRuleSNI() creates a DestinationRule setting up MUTUAL (not ISTIO_MUTUAL) TLS
func serviceToDestinationRuleSNI(rs *v1alpha1.RemoteServiceBinding_RemoteCluster_RemoteService, config istiomodel.Config) *istiomodel.Config {
	return &istiomodel.Config{
		ConfigMeta: istiomodel.ConfigMeta{
			Type:        istiomodel.DestinationRule.Type,
			Group:       istiomodel.DestinationRule.Group + istiomodel.IstioAPIGroupDomain,
			Version:     istiomodel.DestinationRule.Version,
			Name:        fmt.Sprintf("dest-rule-%s-%s", config.Name, meta_v1.NamespaceDefault), // TODO avoid collisions?
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

func rsAliasHostname(rs *v1alpha1.RemoteServiceBinding_RemoteCluster_RemoteService) string {
	return fmt.Sprintf("%s.%s.svc.cluster.global", rs.Name, remoteServiceNamespace(rs))
}

func convertSEPSNI(config istiomodel.Config, sep *v1alpha1.ServiceExpositionPolicy, drs map[string]*istiomodel.Config) ([]istiomodel.Config, error) {
	out := make([]istiomodel.Config, 0)

	for _, remote := range sep.Exposed {
		svcname := remote.Alias
		if svcname == "" {
			svcname = remote.Name
		}

		dr, err := expositionToDestinationRuleSNI(remote, config, drs)
		if err != nil {
			return out, err
		}

		gw, err := expositionToGatewaySNI(remote, config)
		if err != nil {
			return out, err
		}

		vs, err := expositionToVirtualServiceSNI(remote, config)
		if err != nil {
			return out, err
		}

		out = append(out, *dr, *gw, *vs)
	}

	return out, nil
}

// 'drs' maps hostname to DestinationRule and is used to keep track of destinations exposed with different subset and/or alias
func expositionToDestinationRuleSNI(es *v1alpha1.ServiceExpositionPolicy_ExposedService, config istiomodel.Config, drs map[string]*istiomodel.Config) (*istiomodel.Config, error) {
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
				return nil, fmt.Errorf("Exposed subset %q not defined in DestinationRule %s/%s", es.Subset, dr.Namespace, dr.Name)
			}
			labels = origSubset.Labels
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

	// Add the dr to the store so that if another ServiceExpositionPolicy refers to the same
	// service we will modify the DestinationRule we just created.

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

func expositionToGatewaySNI(es *v1alpha1.ServiceExpositionPolicy_ExposedService, config istiomodel.Config) (*istiomodel.Config, error) {
	return &istiomodel.Config{
		ConfigMeta: istiomodel.ConfigMeta{
			Type:    istiomodel.Gateway.Type,
			Group:   istiomodel.Gateway.Group + istiomodel.IstioAPIGroupDomain,
			Version: istiomodel.Gateway.Version,
			Name:    exposedServiceGatewayName(es, config),
			// Namespace:   config.Namespace,
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
					Hosts: []string{fmt.Sprintf("%s.%s.svc.cluster.global", exposedServiceName(es), getNamespace(config))},
					Tls: &v1alpha3.Server_TLSOptions{
						Mode: v1alpha3.Server_TLSOptions_PASSTHROUGH,
					},
				},
			},
			Selector: map[string]string{"istio": "ingressgateway"},
		},
	}, nil
}

// expositionToVirtualServiceSNI() creates a VirtualService with sniHosts
func expositionToVirtualServiceSNI(es *v1alpha1.ServiceExpositionPolicy_ExposedService, config istiomodel.Config) (*istiomodel.Config, error) {
	return &istiomodel.Config{
		ConfigMeta: istiomodel.ConfigMeta{
			Type:    istiomodel.VirtualService.Type,
			Group:   istiomodel.VirtualService.Group + istiomodel.IstioAPIGroupDomain,
			Version: istiomodel.VirtualService.Version,
			Name:    fmt.Sprintf("ingressgateway-to-%s-%s", exposedServiceName(es), getNamespace(config)),
			// Namespace:   config.Namespace,
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
										Number: 80,
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

func esHostnameSNI(config istiomodel.Config, es *v1alpha1.ServiceExpositionPolicy_ExposedService) string {
	return fmt.Sprintf("%s.%s.svc.cluster.global", exposedServiceName(es), meta_v1.NamespaceDefault)
}
