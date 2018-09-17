// Licensed Materials - Property of IBM
// (C) Copyright IBM Corp. 2018. All Rights Reserved.
// US Government Users Restricted Rights - Use, duplication or
// disclosure restricted by GSA ADP Schedule Contract with IBM Corp.
// Copyright 2018 IBM Corporation

package model

import (
	"os"

	istiomodel "istio.io/istio/pilot/pkg/model"

	kube_v1 "k8s.io/api/core/v1"
)

const (
	ProvenanceAnnotationKey = "multicluster.istio.io/provenance"

	IstioSystemNamespace = istiomodel.IstioSystemNamespace // TODO handle non-default installs

	// IstioConversionStyleKey names an exported OS environment variable with value DIRECT_INGRESS or EGRESS_INGRESS
	IstioConversionStyleKey = "MC_STYLE"

	EgressIngressStyle = "EGRESS_INGRESS"
	DirectIngressStyle = "DIRECT_INGRESS"
)

type ClusterInfo interface {
	Ip(name string) string
	Port(name string) uint32
}

func ConvertBindingsAndExposures(mcs []istiomodel.Config, ci ClusterInfo, store istiomodel.ConfigStore) ([]istiomodel.Config, error) {
	if os.Getenv(IstioConversionStyleKey) == DirectIngressStyle {
		istioConfig, k8sConfig, err := ConvertBindingsAndExposuresDirectIngress(mcs, ci, store, []kube_v1.Service{})
		_ = k8sConfig
		return istioConfig, err
	}

	// Default
	return ConvertBindingsAndExposuresEgressIngress(mcs, ci)
}

func ConvertBindingsAndExposures2(mcs []istiomodel.Config, ci ClusterInfo, store istiomodel.ConfigStore, svcs []kube_v1.Service) ([]istiomodel.Config, []kube_v1.Service, error) {
	if os.Getenv(IstioConversionStyleKey) == DirectIngressStyle {
		return ConvertBindingsAndExposuresDirectIngress(mcs, ci, store, svcs)
	}

	// Default
	istioConfig, err := ConvertBindingsAndExposuresEgressIngress(mcs, ci)
	return istioConfig, []kube_v1.Service{}, err
}
