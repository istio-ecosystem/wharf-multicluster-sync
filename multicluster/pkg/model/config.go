package model

import (
	istio "istio.io/istio/pilot/pkg/model"
)

var (
	// ServiceExpositionPolicy describes v1alpha1 multi-cluster exposition policy
	ServiceExpositionPolicy = istio.ProtoSchema{
		Type:        "service-exposition-policy",
		Plural:      "service-exposition-policies",
		Group:       "multicluster",
		Version:     "v1alpha1",
		MessageName: "istio.multicluster.v1alpha1.ServiceExpositionPolicy",
		Validate:    ValidateServiceExpositionPolicy,
	}

	// IstioConfigTypes lists all Istio config types with schemas and validation
	IstioConfigTypes = istio.ConfigDescriptor{
		ServiceExpositionPolicy,
	}
)
