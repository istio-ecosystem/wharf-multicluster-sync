// Licensed Materials - Property of IBM
// (C) Copyright IBM Corp. 2018. All Rights Reserved.
// US Government Users Restricted Rights - Use, duplication or
// disclosure restricted by GSA ADP Schedule Contract with IBM Corp.
// Copyright 2018 IBM Corporation

package model

import (
	"fmt"
	
	"github.ibm.com/istio-research/multicluster-roadmap/api/multicluster/v1alpha1"

	istiomodel "istio.io/istio/pilot/pkg/model"
	"istio.io/api/networking/v1alpha3"

	multierror "github.com/hashicorp/go-multierror"
)

func hostname(svcname string) string {
	return fmt.Sprintf("%s.my-remote.svc.cluster.global", svcname)
}

// serviceToServiceEntry() creates a ServiceEntry pointing to istio-egressgateway
func serviceToServiceEntry(svcname string, config istiomodel.Config) *istiomodel.Config {
	return &istiomodel.Config{
		ConfigMeta: istiomodel.ConfigMeta{
			Type:      istiomodel.ServiceEntry.Type,
			Group:     istiomodel.ServiceEntry.Group,
			Version:   istiomodel.ServiceEntry.Version,
			Name:      fmt.Sprintf("service-entry-%s", config.Name),	// TODO avoid collisions?
			Namespace: config.Namespace,
			// TODO Annotate with provenance
		},
		Spec: &v1alpha3.ServiceEntry{
			Hosts: []string { hostname(svcname) },
			Ports: []*v1alpha3.Port{
				&v1alpha3.Port{
					Number: 80,
					Protocol: "HTTP",
					Name: "http",
				},
			},
			Location: v1alpha3.ServiceEntry_MESH_EXTERNAL,
			Resolution: v1alpha3.ServiceEntry_DNS,
			Endpoints: []*v1alpha3.ServiceEntry_Endpoint{
				&v1alpha3.ServiceEntry_Endpoint {
					Address: "istio-egressgateway.istio-system.svc.cluster.local", // TODO story for non-default Istio install
					Ports: map[string]uint32 { "http": 80 },
				},
			},
		},
	}
}

// serviceToDestinationRule() creates a DestinationRule setting up MUTUAL (not ISTIO_MUTUAL) TLS
func serviceToDestinationRule(svcname string, config istiomodel.Config) *istiomodel.Config {
	return &istiomodel.Config{
		ConfigMeta: istiomodel.ConfigMeta{
			Type:      istiomodel.DestinationRule.Type,
			Group:     istiomodel.DestinationRule.Group,
			Version:   istiomodel.DestinationRule.Version,
			Name:      fmt.Sprintf("dest-rule-%s-my-remote", config.Name),	// TODO avoid collisions?
			Namespace: config.Namespace,
			// TODO Annotate with provenance
		},
		Spec: &v1alpha3.DestinationRule{
			Host: hostname(svcname),
			TrafficPolicy: &v1alpha3.TrafficPolicy{
				Tls: &v1alpha3.TLSSettings{
					Mode: v1alpha3.TLSSettings_MUTUAL,
					ClientCertificate: "/etc/certs/cert-chain.pem",
					PrivateKey: "/etc/certs/key.pem",
					CaCertificates: "/etc/certs/root-cert.pem",
					Sni: hostname(svcname),
				},
			},
		},
	}
}

// serviceToGateway() creates a Gateway with TLS PASSTHROUGH
func serviceToGateway(svcname string, config istiomodel.Config) *istiomodel.Config {
	return &istiomodel.Config{
		ConfigMeta: istiomodel.ConfigMeta{
			Type:      istiomodel.Gateway.Type,
			Group:     istiomodel.Gateway.Group,
			Version:   istiomodel.Gateway.Version,
			Name:      fmt.Sprintf("istio-egressgateway-%s-my-remote", config.Name),	// TODO avoid collisions?
			Namespace: config.Namespace,
			// TODO Annotate with provenance
		},
		Spec: &v1alpha3.Gateway{
			Servers: []*v1alpha3.Server{
				&v1alpha3.Server{
					Port: &v1alpha3.Port{
						Number: 80,
						Protocol: "TLS",
						Name: fmt.Sprintf("%s-my-remote-%d", svcname, 80),
					},
					Hosts: []string { hostname(svcname) },
					Tls: &v1alpha3.Server_TLSOptions{
						Mode: v1alpha3.Server_TLSOptions_PASSTHROUGH,
					},
				},
			},
			Selector: map[string]string {"istio": "egressgateway"}, // TODO handle non-default install options?
		},
	}
}

func gatewayName(config istiomodel.Config) string {
	return fmt.Sprintf("istio-egressgateway-%s-my-remote", config.Name)	// TODO avoid collisions?
}

// serviceToVirtualService() creates a VirtualService with sniHosts
func serviceToVirtualService(cluster string, svcname string, config istiomodel.Config) *istiomodel.Config {
	return &istiomodel.Config{
		ConfigMeta: istiomodel.ConfigMeta{
			Type:      istiomodel.VirtualService.Type,
			Group:     istiomodel.VirtualService.Group,
			Version:   istiomodel.VirtualService.Version,
			Name:      gatewayName(config),
			Namespace: config.Namespace,
			// TODO Annotate with provenance
		},
		Spec: &v1alpha3.VirtualService{
			Hosts: []string { hostname(svcname) },
			Gateways: []string { gatewayName(config) },
			Tls: []*v1alpha3.TLSRoute{
				&v1alpha3.TLSRoute{
					Match: []*v1alpha3.TLSMatchAttributes{
						&v1alpha3.TLSMatchAttributes{
							SniHosts: []string { hostname(svcname) },
							Port: 80,
						},
					},
					Route: []*v1alpha3.DestinationWeight{
						&v1alpha3.DestinationWeight{
							Destination: &v1alpha3.Destination{
								Host: cluster,
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
func clusterToServiceEntry(cluster string, ip string, port uint32, config istiomodel.Config) *istiomodel.Config {
	return &istiomodel.Config{
		ConfigMeta: istiomodel.ConfigMeta{
			Type:      istiomodel.ServiceEntry.Type,
			Group:     istiomodel.ServiceEntry.Group,
			Version:   istiomodel.ServiceEntry.Version,
			Name:      fmt.Sprintf("service-entry-ingress-gateway-%s", cluster),
			Namespace: config.Namespace,
			// TODO Annotate with provenance
		},
		Spec: &v1alpha3.ServiceEntry{
			Hosts: []string { cluster },
			Addresses: []string { "127.8.8.8" }, // dummy
			Ports: []*v1alpha3.Port{
				&v1alpha3.Port{
					Number: 80,
					Protocol: "TCP",
					Name: "tcp",
				},
			},
			Location: v1alpha3.ServiceEntry_MESH_EXTERNAL,
			Resolution: v1alpha3.ServiceEntry_DNS,
			Endpoints: []*v1alpha3.ServiceEntry_Endpoint{
				&v1alpha3.ServiceEntry_Endpoint {
					Address: ip,
					Ports: map[string]uint32 { "tcp": port },
				},
			},
		},
	}
}

func convertRSB(config istiomodel.Config, rsb *v1alpha1.RemoteServiceBinding) ([]istiomodel.Config, error) {
	out := make([]istiomodel.Config, 0)

	for _, remote := range rsb.Remote {
		ip := "127.0.0.1"	// TODO this should be looked up using cluster naming mechanism
		port := 80			// TODO This should be looked up using cluster naming mechanism
		for _, svc := range remote.Services {
			svcname := svc.Alias
			if svcname == "" {
				svcname = svc.Name
			}
			
			out = append(out, *serviceToServiceEntry(svcname, config))
			out = append(out, *serviceToDestinationRule(svcname, config))
			out = append(out, *serviceToGateway(svcname, config))
			out = append(out, *serviceToVirtualService(remote.Cluster, svcname, config))
		}
		out = append(out, *clusterToServiceEntry(remote.Cluster, ip, uint32(port), config))
	}
	
	return out, nil
}

func convertSEP(config istiomodel.Config, sep *v1alpha1.ServiceExpositionPolicy) ([]istiomodel.Config, error) {
	out := make([]istiomodel.Config, 0)

	// TODO
	
	return out, nil
}

func ConvertBindingsAndExposures(mcs []istiomodel.Config) ([]istiomodel.Config, error) {
	out := make([]istiomodel.Config, 0)
	
	for _, mc := range mcs {
		var istio []istiomodel.Config
		var err error
		rsb, ok := mc.Spec.(*v1alpha1.RemoteServiceBinding)
		if ok {
			istio, err = convertRSB(mc, rsb)
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