// Package model provides an implementation of the MCConfigStore.  It also
// provides conversion from ServiceExpositionPolicy and
// RemoteServiceBinding to Istio rules that express the policy.
package model

import (
	istio "istio.io/istio/pilot/pkg/model"
)

// MCConfigStore is a specialized interface to access config store using
// Multi-Cluster configuration types
type MCConfigStore interface {
	istio.ConfigStore

	// ServiceExpositionPolicies lists all ServiceExpositionPolicy entries
	ServiceExpositionPolicies() []istio.Config

	// RemoteServiceBindings lists all RemoteServiceBinding entries
	RemoteServiceBindings() []istio.Config
}

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

	// RemoteServiceBinding describes v1alpha1 multi-cluster remote service binding
	RemoteServiceBinding = istio.ProtoSchema{
		Type:        "remote-service-binding",
		Plural:      "remote-service-bindings",
		Group:       "multicluster",
		Version:     "v1alpha1",
		MessageName: "istio.multicluster.v1alpha1.RemoteServiceBinding",
		Validate:    ValidateRemoteServiceBinding,
	}

	// MultiClusterConfigTypes lists all Istio config types with schemas and validation
	MultiClusterConfigTypes = istio.ConfigDescriptor{
		ServiceExpositionPolicy,
		RemoteServiceBinding,
	}
)

// mcConfigStore provides a simple adapter for Multi-Cluster configuration types
// from the generic config registry
type mcConfigStore struct {
	istio.ConfigStore
}

// MakeMCStore creates a wrapper around a store
func MakeMCStore(store istio.ConfigStore) MCConfigStore {
	return &mcConfigStore{store}
}

// ServiceExpositionPolicies will return all ServiceExpositionPolicy entries
// from the store
func (store *mcConfigStore) ServiceExpositionPolicies() []istio.Config {
	configs, err := store.List(ServiceExpositionPolicy.Type, istio.NamespaceAll)
	if err != nil {
		return nil
	}
	return configs
}

// RemoteServiceBindings will return all RemoteServiceBinding entries
// from the store
func (store *mcConfigStore) RemoteServiceBindings() []istio.Config {
	configs, err := store.List(RemoteServiceBinding.Type, istio.NamespaceAll)
	if err != nil {
		return nil
	}
	return configs
}
