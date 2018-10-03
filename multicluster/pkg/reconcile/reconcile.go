// Licensed Materials - Property of IBM
// (C) Copyright IBM Corp. 2018. All Rights Reserved.
// US Government Users Restricted Rights - Use, duplication or
// disclosure restricted by GSA ADP Schedule Contract with IBM Corp.
// Copyright 2018 IBM Corporation

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
			if !reflect.DeepEqual(svc.Spec, orig.Spec) {
				svcModifications = append(svcModifications, svc)
			}
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

	// TODO: if a K8s Service was created by us, and has no local selector matches, delete it
	_ = svcs

	return &ConfigChanges{
		Deletions: outDeletions,
	}, err
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
