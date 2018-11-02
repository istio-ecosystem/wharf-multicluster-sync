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

package reconcile

import (
	"fmt"
	"reflect"

	multierror "github.com/hashicorp/go-multierror"

	istiomodel "istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pkg/log"

	"github.com/istio-ecosystem/wharf-multicluster-sync/multicluster/pkg/model"

	kube_v1 "k8s.io/api/core/v1"
)

// KubernetesChanges lists changes needed to bring Kubernetes in line with multicluster desired state
type KubernetesChanges struct {
	Additions     []kube_v1.Service
	Modifications []kube_v1.Service
	Deletions     []kube_v1.Service
}

// ConfigChanges lists changes needed to bring Istio in line with multicluster desired state
type ConfigChanges struct {
	Additions     []istiomodel.Config
	Modifications []istiomodel.Config
	Deletions     []istiomodel.Config
	Kubernetes    *KubernetesChanges
}

type reconciler struct {
	store       istiomodel.ConfigStore
	services    []kube_v1.Service
	clusterInfo model.ClusterInfo
}

// Reconciler merges new multicluster desired state config with existing Istio and K8s configuration producing the desired state
type Reconciler interface {
	AddMulticlusterConfig(config istiomodel.Config) (*ConfigChanges, error)
	ModifyMulticlusterConfig(config istiomodel.Config) (*ConfigChanges, error)
	DeleteMulticlusterConfig(config istiomodel.Config) (*ConfigChanges, error)
}

// NewReconciler creates a Reconciler to merge existing configuration with Multicluster configuration
func NewReconciler(store istiomodel.ConfigStore, services []kube_v1.Service, clusterInfo model.ClusterInfo) Reconciler {
	return &reconciler{
		store:       store,
		services:    services,
		clusterInfo: clusterInfo,
	}
}

// AddMulticlusterConfig takes an Istio config store and a new RemoteServiceBinding or ServiceExpositionPolicy
// and returns the new and modified Istio configurations needed to implement the desired multicluster config.
func (r *reconciler) AddMulticlusterConfig(newconfig istiomodel.Config) (*ConfigChanges, error) {

	istioConfigs, svcs, err := model.ConvertBindingsAndExposures2(
		[]istiomodel.Config{newconfig}, r.clusterInfo, r.store, r.services)
	if err != nil {
		return nil, err
	}

	outAdditions := make([]istiomodel.Config, 0)
	outModifications := make([]istiomodel.Config, 0)
	for _, istioConfig := range istioConfigs {
		orig, ok := r.store.Get(istioConfig.Type, istioConfig.Name, getNamespace(istioConfig))
		if !ok {
			outAdditions = append(outAdditions, istioConfig)
		} else {
			if !reflect.DeepEqual(istioConfig.Spec, orig.Spec) {
				outModifications = append(outModifications, istioConfig)
			}
		}
	}

	origSvcs := indexServices(r.services, svcIndex)
	svcAdditions := make([]kube_v1.Service, 0)
	svcModifications := make([]kube_v1.Service, 0)
	for _, svc := range svcs {
		orig, ok := origSvcs[svcIndex(svc)]
		if !ok {
			svcAdditions = append(svcAdditions, svc)
		} else {
			// Compare, but don't include generated immutable field ClusterIP in comparison
			origNoIP := orig.Spec
			origNoIP.ClusterIP = ""
			if !reflect.DeepEqual(svc.Spec, origNoIP) {
				// New version is different in some way besides ClusterIP.  Make a new one,
				// but use the UID and ClusterIP of the old one so that we survive K8s
				// immutability requirement on ClusterIP.
				newSpec := svc.Spec
				origSpec := orig.Spec
				newSpec.ClusterIP = origSpec.ClusterIP
				svc.UID = orig.UID
				svcModifications = append(svcModifications, svc)
			}
			// TODO merge Annotations if multiple remote clusters offer service
		}
	}

	return &ConfigChanges{
		Additions:     outAdditions,
		Modifications: outModifications,
		Kubernetes: &KubernetesChanges{
			Additions:     svcAdditions,
			Modifications: svcModifications,
		},
	}, nil
}

// ModifyMulticlusterConfig takes an Istio config store and a modified RemoteServiceBinding or ServiceExpositionPolicy
// and returns the new and modified Istio configurations needed to implement the desired multicluster config.
func (r *reconciler) ModifyMulticlusterConfig(config istiomodel.Config) (*ConfigChanges, error) {

	// Modifying RSB or SEP usually only modifies Istio config, but it might generate new if Istio config was deleted.
	// So don't bother validating that Istio and K8s Services exist, just do what "Add" would do.
	return r.AddMulticlusterConfig(config)
}

// DeleteMulticlusterConfig takes an Istio config store and a deleted RemoteServiceBinding or ServiceExpositionPolicy
// and returns the Istio configurations that should be removed to disable the multicluster config.
// Only the Type, Name, and Namespace of the output configs is guaranteed usable.
func (r *reconciler) DeleteMulticlusterConfig(config istiomodel.Config) (*ConfigChanges, error) {

	var err error
	istioConfigs, svcs, err := model.ConvertBindingsAndExposures2(
		[]istiomodel.Config{config}, r.clusterInfo, r.store, r.services)
	if err != nil {
		return nil, err
	}

	outDeletions := make([]istiomodel.Config, 0)
	for _, istioConfig := range istioConfigs {
		orig, ok := r.store.Get(istioConfig.Type, istioConfig.Name, getNamespace(istioConfig))
		if !ok {
			err = multierror.Append(err, fmt.Errorf("%s %s.%s should have been realized by %s %s.%s; skipping",
				config.Type, config.Name, config.Namespace,
				istioConfig.Type, istioConfig.Name, getNamespace(istioConfig)))
		} else {
			// Only delete if our annotation is present
			_, ok := orig.Annotations[model.ProvenanceAnnotationKey]
			if ok {
				istioConfig.Spec = nil // Don't let caller see the details, their job is to delete based on Kind and Name
				outDeletions = append(outDeletions, istioConfig)
			} else {
				log.Infof("Ignoring unprovenanced %s %s.%s when reconciling deletion",
					istioConfig.Type, istioConfig.Name, getNamespace(istioConfig))
			}
		}
	}

	origSvcs := indexServices(r.services, svcIndex)
	svcModifications := make([]kube_v1.Service, 0)
	svcDeletions := make([]kube_v1.Service, 0)
	for _, svc := range svcs {
		orig, ok := origSvcs[svcIndex(svc)]
		if ok {
			// There is a service.  If there is an annotation for us, and it is the only one,
			// delete the service.  If there is annotations for multiple remotes, modify to remove this remote.
			// Only delete if our annotation is present
			origAnn, ok := orig.Annotations[model.ProvenanceAnnotationKey]
			if ok {
				if lastRemote(origAnn, config) {
					svcDeletions = append(svcDeletions, svc)
				} else {
					newSpec := svc.Spec
					origSpec := orig.Spec
					newSpec.ClusterIP = origSpec.ClusterIP
					svc.UID = orig.UID
					svcModifications = append(svcModifications, svc)
				}
			} else {
				log.Infof("Ignoring unprovenanced K8s Service %s.%s when reconciling deletion",
					svc.Name, getK8sNamespace(svc))
			}
		}
	}

	return &ConfigChanges{
		Deletions: outDeletions,
		Kubernetes: &KubernetesChanges{
			Modifications: svcModifications,
			Deletions:     svcDeletions,
		},
	}, err
}

// lastRemote() returns true for annotations that only refer to this cluster.
func lastRemote(annotation string, config istiomodel.Config) bool {
	return annotation == model.ProvenanceAnnotation(config)
}

func indexServices(svcs []kube_v1.Service, indexFunc func(config kube_v1.Service) string) map[string]kube_v1.Service {
	out := make(map[string]kube_v1.Service)
	for _, svc := range svcs {
		out[indexFunc(svc)] = svc
	}
	return out
}

func svcIndex(config kube_v1.Service) string {
	return fmt.Sprintf("Service+%s+%s", config.Namespace, config.Name)
}

func getNamespace(config istiomodel.Config) string {
	if config.Namespace != "" {
		return config.Namespace
	}
	// TODO incorporate parsing $KUBECONFIG similar to routine in istio.io/istio/istioctl/cmd/istioctl/main.go
	return config.Namespace // kube_v1.NamespaceDefault
}

func getK8sNamespace(svc kube_v1.Service) string {
	if svc.Namespace != "" {
		return svc.Namespace
	}
	// TODO incorporate parsing $KUBECONFIG similar to routine in istio.io/istio/istioctl/cmd/istioctl/main.go
	return kube_v1.NamespaceDefault
}
