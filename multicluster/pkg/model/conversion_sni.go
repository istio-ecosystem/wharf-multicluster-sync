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

func ConvertBindingsAndExposuresSNI(mcs []istiomodel.Config, ci ClusterInfo) ([]istiomodel.Config, error) {
	out := make([]istiomodel.Config, 0)

	for _, mc := range mcs {
		var istio []istiomodel.Config
		var err error
		rsb, ok := mc.Spec.(*v1alpha1.RemoteServiceBinding)
		if ok {
			istio, err = convertRSBSNI(mc, rsb, ci)
		}
		sep, ok := mc.Spec.(*v1alpha1.ServiceExpositionPolicy)
		if ok {
			istio, err = convertSEPSNI(mc, sep)
		}
		if err != nil {
			return out, multierror.Prefix(err, "Could not convert")
		}
		out = append(out, istio...)
	}

	return out, nil
}

func convertRSBSNI(config istiomodel.Config, rsb *v1alpha1.RemoteServiceBinding, ci ClusterInfo) ([]istiomodel.Config, error) {
	out := make([]istiomodel.Config, 0)

	for _, remote := range rsb.Remote {
		for _, svc := range remote.Services {
			out = append(out, *serviceToServiceEntrySNI(svc, config))
			out = append(out, *serviceToDestinationRuleSNI(svc, config))
		}
	}

	return out, nil
}

// serviceToServiceEntry() creates a ServiceEntry pointing to istio-egressgateway
func serviceToServiceEntrySNI(rs *v1alpha1.RemoteServiceBinding_RemoteCluster_RemoteService, config istiomodel.Config) *istiomodel.Config {
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
					Address: fmt.Sprintf("istio-egressgateway.%s.svc.cluster.local", IstioSystemNamespace),
					Ports:   map[string]uint32{"tcp": 80},
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
					Sni:               rsHostname(rs),
				},
			},
		},
	}
}

func convertSEPSNI(config istiomodel.Config, sep *v1alpha1.ServiceExpositionPolicy) ([]istiomodel.Config, error) {
	out := make([]istiomodel.Config, 0)

	for _, remote := range sep.Exposed {
		svcname := remote.Alias
		if svcname == "" {
			svcname = remote.Name
		}

		dr, err := expositionToDestinationRuleSNI(remote, config)
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

func expositionToDestinationRuleSNI(es *v1alpha1.ServiceExpositionPolicy_ExposedService, config istiomodel.Config) (*istiomodel.Config, error) {
	return &istiomodel.Config{
		ConfigMeta: istiomodel.ConfigMeta{
			Type:    istiomodel.DestinationRule.Type,
			Group:   istiomodel.DestinationRule.Group + istiomodel.IstioAPIGroupDomain,
			Version: istiomodel.DestinationRule.Version,
			Name:    fmt.Sprintf("dest-rule-%s-default-notls", es.Name), // TODO avoid collisions?
			// Namespace:   config.Namespace,
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
			Name:    fmt.Sprintf("ingressgateway-to-%s-%s", es.Name, getNamespace(config)),
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

func esHostnameSNI(config istiomodel.Config, es *v1alpha1.ServiceExpositionPolicy_ExposedService) string {
	return fmt.Sprintf("%s.%s.svc.cluster.global", exposedServiceName(es), meta_v1.NamespaceDefault)
}
