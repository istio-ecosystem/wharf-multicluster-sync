// (C) Copyright IBM Corp. 2018. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package model

import (
	"os"

	istiomodel "istio.io/istio/pilot/pkg/model"

	kube_v1 "k8s.io/api/core/v1"
)

const (
	// ProvenanceAnnotationKey is the key to an annotation that maps created config back to multicluster desired state CRD
	ProvenanceAnnotationKey = "multicluster.istio.io/provenance"

	// IstioSystemNamespace is "istio-system", the namespace where the Istio components run
	IstioSystemNamespace = istiomodel.IstioSystemNamespace // TODO handle non-default installs

	// IstioConversionStyleKey names an exported OS environment variable with value DIRECT_INGRESS or EGRESS_INGRESS
	IstioConversionStyleKey = "MC_STYLE"

	// EgressIngressStyle is a value for IstioConversionStyleKey that requests the agent create
	// Istio configuration that flows through an Egress
	EgressIngressStyle = "EGRESS_INGRESS"
	// DirectIngressStyle is a value for IstioConversionStyleKey that requests the agent create
	// Istio configuration that communicates directly to the remote IngressGateway
	DirectIngressStyle = "DIRECT_INGRESS"
)

// ClusterInfo gets the IP and port for a cluster's ingress
type ClusterInfo interface {
	// Gateway gives the host/port used for exposing
	Gateway() (string, uint32)

	// IP gives the IP address (typically for binding)
	IP(name string) string

	// Port gives the remote port (typically for binding)
	Port(name string) uint32
}

// ConvertBindingsAndExposures is deprecated
func ConvertBindingsAndExposures(mcs []istiomodel.Config, ci ClusterInfo, store istiomodel.ConfigStore) ([]istiomodel.Config, error) {
	if os.Getenv(IstioConversionStyleKey) == DirectIngressStyle {
		istioConfig, k8sConfig, err := ConvertBindingsAndExposuresDirectIngress(mcs, ci, store, []kube_v1.Service{})
		_ = k8sConfig
		return istioConfig, err
	}

	// Default
	return ConvertBindingsAndExposuresEgressIngress(mcs, ci)
}

// ConvertBindingsAndExposures2 converts desired multicluster state into Kubernetes and Istio state
func ConvertBindingsAndExposures2(mcs []istiomodel.Config, ci ClusterInfo, store istiomodel.ConfigStore, svcs []kube_v1.Service) ([]istiomodel.Config, []kube_v1.Service, error) { // nolint: lll
	if os.Getenv(IstioConversionStyleKey) == DirectIngressStyle {
		return ConvertBindingsAndExposuresDirectIngress(mcs, ci, store, svcs)
	}

	// Default
	istioConfig, err := ConvertBindingsAndExposuresEgressIngress(mcs, ci)
	return istioConfig, []kube_v1.Service{}, err
}
