// Licensed Materials - Property of IBM
// (C) Copyright IBM Corp. 2018. All Rights Reserved.
// US Government Users Restricted Rights - Use, duplication or
// disclosure restricted by GSA ADP Schedule Contract with IBM Corp.
// Copyright 2018 IBM Corporation

package model

import (
	"fmt"
	"strings"

	"github.com/istio-ecosystem/wharf-multicluster-sync/api/multicluster/v1alpha1"

	"istio.io/api/networking/v1alpha3"
	istiomodel "istio.io/istio/pilot/pkg/model"

	multierror "github.com/hashicorp/go-multierror"
	"k8s.io/api/core/v1"
)

func remoteServiceNamespace(rs *v1alpha1.RemoteServiceBinding_RemoteCluster_RemoteService) string {
	if rs.Namespace != "" {
		return rs.Namespace
	}

	return v1.NamespaceDefault
}

func rsHostname(rs *v1alpha1.RemoteServiceBinding_RemoteCluster_RemoteService) string {
	// We give a .local rather than .global hostname so that we can use a K8s Service
	// to create the DNS and keep apps from knowing the communication is multi-cluster
	return fmt.Sprintf("%s.%s.svc.cluster.local", remoteServiceName(rs), remoteServiceNamespace(rs))
}

// serviceToServiceEntry() creates a ServiceEntry pointing to istio-egressgateway
func serviceToServiceEntry(rs *v1alpha1.RemoteServiceBinding_RemoteCluster_RemoteService, config istiomodel.Config) *istiomodel.Config {
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
			Resolution: v1alpha3.ServiceEntry_DNS,
			Endpoints: []*v1alpha3.ServiceEntry_Endpoint{
				&v1alpha3.ServiceEntry_Endpoint{
					Address: fmt.Sprintf("istio-egressgateway.%s.svc.cluster.local", IstioSystemNamespace),
					Ports:   map[string]uint32{"http": 80},
				},
			},
		},
	}
}

// serviceToDestinationRule() creates a DestinationRule setting up MUTUAL (not ISTIO_MUTUAL) TLS
func serviceToDestinationRule(rs *v1alpha1.RemoteServiceBinding_RemoteCluster_RemoteService, config istiomodel.Config) *istiomodel.Config {
	return &istiomodel.Config{
		ConfigMeta: istiomodel.ConfigMeta{
			Type:        istiomodel.DestinationRule.Type,
			Group:       istiomodel.DestinationRule.Group + istiomodel.IstioAPIGroupDomain,
			Version:     istiomodel.DestinationRule.Version,
			Name:        fmt.Sprintf("dest-rule-%s-%s", config.Name, rs.Namespace), // TODO avoid collisions?
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
					Sni:               rsHostname(rs),
				},
			},
		},
	}
}

func remoteServiceName(rs *v1alpha1.RemoteServiceBinding_RemoteCluster_RemoteService) string {
	if rs.Alias != "" {
		return rs.Alias
	}

	return rs.Name
}

func bindingGatewayName(rs *v1alpha1.RemoteServiceBinding_RemoteCluster_RemoteService) string {
	return fmt.Sprintf("istio-egressgateway-%s-%s", rs.Name, rs.Namespace)
}

// serviceToGateway() creates a Gateway with TLS PASSTHROUGH
func serviceToGateway(rs *v1alpha1.RemoteServiceBinding_RemoteCluster_RemoteService, config istiomodel.Config) *istiomodel.Config {
	return &istiomodel.Config{
		ConfigMeta: istiomodel.ConfigMeta{
			Type:        istiomodel.Gateway.Type,
			Group:       istiomodel.Gateway.Group + istiomodel.IstioAPIGroupDomain,
			Version:     istiomodel.Gateway.Version,
			Name:        bindingGatewayName(rs), // TODO avoid collisions?
			Namespace:   config.Namespace,
			Annotations: annotations(config),
		},
		Spec: &v1alpha3.Gateway{
			Servers: []*v1alpha3.Server{
				&v1alpha3.Server{
					Port: &v1alpha3.Port{
						Number:   80,
						Protocol: "TLS",
						Name:     fmt.Sprintf("%s-%s-%d", remoteServiceName(rs), rs.Namespace, 80),
					},
					Hosts: []string{rsHostname(rs)},
					Tls: &v1alpha3.Server_TLSOptions{
						Mode: v1alpha3.Server_TLSOptions_PASSTHROUGH,
					},
				},
			},
			Selector: map[string]string{"istio": "egressgateway"},
		},
	}
}

// serviceToVirtualService() creates a VirtualService with sniHosts
func serviceToVirtualService(remote *v1alpha1.RemoteServiceBinding_RemoteCluster,
	rs *v1alpha1.RemoteServiceBinding_RemoteCluster_RemoteService,
	config istiomodel.Config) *istiomodel.Config {
	return &istiomodel.Config{
		ConfigMeta: istiomodel.ConfigMeta{
			Type:        istiomodel.VirtualService.Type,
			Group:       istiomodel.VirtualService.Group + istiomodel.IstioAPIGroupDomain,
			Version:     istiomodel.VirtualService.Version,
			Name:        fmt.Sprintf("egressgateway-to-ingressgateway-%s-%s", rs.Name, rs.Namespace), // TODO avoid collisions?
			Namespace:   config.Namespace,
			Annotations: annotations(config),
		},
		Spec: &v1alpha3.VirtualService{
			Hosts:    []string{rsHostname(rs)},
			Gateways: []string{bindingGatewayName(rs)},
			Tls: []*v1alpha3.TLSRoute{
				&v1alpha3.TLSRoute{
					Match: []*v1alpha3.TLSMatchAttributes{
						&v1alpha3.TLSMatchAttributes{
							SniHosts: []string{rsHostname(rs)},
							Port:     80,
						},
					},
					Route: []*v1alpha3.DestinationWeight{
						&v1alpha3.DestinationWeight{
							Destination: &v1alpha3.Destination{
								Host: clusterHostname(remote),
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
	}
}

// clusterToServiceEntry() creates a ServiceEntry pointing to a remote cluster
func clusterToServiceEntry(remote *v1alpha1.RemoteServiceBinding_RemoteCluster,
	ip string, port uint32, config istiomodel.Config) *istiomodel.Config {
	return &istiomodel.Config{
		ConfigMeta: istiomodel.ConfigMeta{
			Type:        istiomodel.ServiceEntry.Type,
			Group:       istiomodel.ServiceEntry.Group + istiomodel.IstioAPIGroupDomain,
			Version:     istiomodel.ServiceEntry.Version,
			Name:        fmt.Sprintf("service-entry-ingress-gateway-%s", remote.Cluster),
			Namespace:   config.Namespace,
			Annotations: annotations(config),
		},
		Spec: &v1alpha3.ServiceEntry{
			Hosts:     []string{clusterHostname(remote)},
			Addresses: []string{"127.8.8.8"}, // dummy
			Ports: []*v1alpha3.Port{
				&v1alpha3.Port{
					Number:   80,
					Protocol: "TCP",
					Name:     "tcp",
				},
			},
			Location:   v1alpha3.ServiceEntry_MESH_EXTERNAL,
			Resolution: v1alpha3.ServiceEntry_DNS,
			Endpoints: []*v1alpha3.ServiceEntry_Endpoint{
				&v1alpha3.ServiceEntry_Endpoint{
					Address: ip,
					Ports:   map[string]uint32{"tcp": port},
				},
			},
		},
	}
}

func convertRSB(config istiomodel.Config, rsb *v1alpha1.RemoteServiceBinding, ci ClusterInfo) ([]istiomodel.Config, error) {
	out := make([]istiomodel.Config, 0)

	for _, remote := range rsb.Remote {
		for _, svc := range remote.Services {
			out = append(out, *serviceToServiceEntry(svc, config))
			out = append(out, *serviceToDestinationRule(svc, config))
			out = append(out, *serviceToGateway(svc, config))
			out = append(out, *serviceToVirtualService(remote, svc, config))
		}
		out = append(out, *clusterToServiceEntry(remote, ci.IP(remote.Cluster), ci.Port(remote.Cluster), config))
	}

	return out, nil
}

func clusterHostname(remote *v1alpha1.RemoteServiceBinding_RemoteCluster) string {
	return fmt.Sprintf("%s.myorg", strings.ToLower(remote.Cluster))
}

func expositionToDestinationRule(es *v1alpha1.ServiceExpositionPolicy_ExposedService, config istiomodel.Config) (*istiomodel.Config, error) {
	return &istiomodel.Config{
		ConfigMeta: istiomodel.ConfigMeta{
			Type:        istiomodel.DestinationRule.Type,
			Group:       istiomodel.DestinationRule.Group + istiomodel.IstioAPIGroupDomain,
			Version:     istiomodel.DestinationRule.Version,
			Name:        fmt.Sprintf("dest-rule-%s-default-notls", es.Name), // TODO avoid collisions?
			Namespace:   config.Namespace,
			Annotations: annotations(config),
		},
		Spec: &v1alpha3.DestinationRule{
			Host: fmt.Sprintf("%s.default.svc.cluster.local", es.Name),
			Subsets: []*v1alpha3.Subset{
				&v1alpha3.Subset{
					Name: "notls",
					TrafficPolicy: &v1alpha3.TrafficPolicy{
						Tls: &v1alpha3.TLSSettings{
							Mode: v1alpha3.TLSSettings_DISABLE,
						},
					},
				},
			},
		},
	}, nil
}

func exposedServiceName(es *v1alpha1.ServiceExpositionPolicy_ExposedService) string {
	if es.Alias != "" {
		return es.Alias
	}

	return es.Name
}

func exposedServiceGatewayName(es *v1alpha1.ServiceExpositionPolicy_ExposedService, config istiomodel.Config) string {
	return fmt.Sprintf("istio-ingressgateway-%s-%s", exposedServiceName(es), getNamespace(config)) // TODO avoid collisions?
}

func getNamespace(config istiomodel.Config) string {
	if config.Namespace != "" {
		return config.Namespace
	}

	return v1.NamespaceDefault
}

func expositionToGateway(es *v1alpha1.ServiceExpositionPolicy_ExposedService, config istiomodel.Config) (*istiomodel.Config, error) {
	return &istiomodel.Config{
		ConfigMeta: istiomodel.ConfigMeta{
			Type:        istiomodel.Gateway.Type,
			Group:       istiomodel.Gateway.Group + istiomodel.IstioAPIGroupDomain,
			Version:     istiomodel.Gateway.Version,
			Name:        exposedServiceGatewayName(es, config),
			Namespace:   config.Namespace,
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

func esHostname(config istiomodel.Config, es *v1alpha1.ServiceExpositionPolicy_ExposedService) string {
	// We give a .local rather than .global hostname so that we can use a K8s Service
	// to create the DNS and keep apps from knowing the communication is multi-cluster
	return fmt.Sprintf("%s.%s.svc.cluster.local", exposedServiceName(es), getNamespace(config))
}

// expositionToVirtualService() creates a VirtualService with sniHosts
func expositionToVirtualService(es *v1alpha1.ServiceExpositionPolicy_ExposedService, config istiomodel.Config) (*istiomodel.Config, error) {
	return &istiomodel.Config{
		ConfigMeta: istiomodel.ConfigMeta{
			Type:        istiomodel.VirtualService.Type,
			Group:       istiomodel.VirtualService.Group + istiomodel.IstioAPIGroupDomain,
			Version:     istiomodel.VirtualService.Version,
			Name:        fmt.Sprintf("ingressgateway-to-%s-%s", es.Name, getNamespace(config)),
			Namespace:   config.Namespace,
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
								Host:   fmt.Sprintf("%s.%s.svc.cluster.local", es.Name, getNamespace(config)),
								Subset: "notls",
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

func convertSEP(config istiomodel.Config, sep *v1alpha1.ServiceExpositionPolicy) ([]istiomodel.Config, error) {
	out := make([]istiomodel.Config, 0)

	for _, remote := range sep.Exposed {
		dr, err := expositionToDestinationRule(remote, config)
		if err != nil {
			return out, err
		}

		gw, err := expositionToGateway(remote, config)
		if err != nil {
			return out, err
		}

		vs, err := expositionToVirtualService(remote, config)
		if err != nil {
			return out, err
		}

		out = append(out, *dr, *gw, *vs)
	}

	return out, nil
}

// ConvertBindingsAndExposuresEgressIngress converts multicluster desired state into Istio state
func ConvertBindingsAndExposuresEgressIngress(mcs []istiomodel.Config, ci ClusterInfo) ([]istiomodel.Config, error) {
	out := make([]istiomodel.Config, 0)

	for _, mc := range mcs {
		var istio []istiomodel.Config
		var err error
		rsb, ok := mc.Spec.(*v1alpha1.RemoteServiceBinding)
		if ok {
			istio, err = convertRSB(mc, rsb, ci)
		}
		sep, ok := mc.Spec.(*v1alpha1.ServiceExpositionPolicy)
		if ok {
			istio, err = convertSEP(mc, sep)
		}
		if err != nil {
			return out, multierror.Prefix(err, "Could not convert")
		}
		out = append(out, istio...)
	}

	return out, nil
}

func provenanceAnnotation(config istiomodel.Config) string {
	return fmt.Sprintf("%s.%s", namespace(config), config.Name)
}

func namespace(config istiomodel.Config) string {
	if config.Namespace != "" {
		return config.Namespace
	}
	return v1.NamespaceDefault
}

func annotations(config istiomodel.Config) map[string]string {
	return map[string]string{
		ProvenanceAnnotationKey: provenanceAnnotation(config),
	}
}
