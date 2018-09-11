// Licensed Materials - Property of IBM
// (C) Copyright IBM Corp. 2018. All Rights Reserved.
// US Government Users Restricted Rights - Use, duplication or
// disclosure restricted by GSA ADP Schedule Contract with IBM Corp.
// Copyright 2018 IBM Corporation

package model

import (
	"os"

	istiomodel "istio.io/istio/pilot/pkg/model"
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
		return ConvertBindingsAndExposuresDirectIngress(mcs, ci, store)
	}

	// Default
	return ConvertBindingsAndExposuresEgressIngress(mcs, ci)
}
