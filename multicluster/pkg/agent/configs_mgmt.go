package agent

import (
	"github.com/istio-ecosystem/wharf-multicluster-sync/multicluster/pkg/reconcile"

	"istio.io/istio/pilot/pkg/model"
	kubecfg "istio.io/istio/pkg/kube"
	"istio.io/istio/pkg/log"

	"k8s.io/client-go/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// ConfigsManagement provides functions for handling changes in Multi-cluster
// configs. Managing the life-cycle of MC configs by calling the functions here
// will make sure all reconciled resources will also be handled accordingly.
type ConfigsManagement struct {
	istioStore    model.ConfigStore
	kubeconfig    string
	context       string
	clusterConfig *ClusterConfig
}

// NewConfigsManagement creates a new instance for the configs management
func NewConfigsManagement(kubeconfig, context string, istioStore model.ConfigStore, clusterConfig *ClusterConfig) *ConfigsManagement {
	return &ConfigsManagement{
		istioStore:    istioStore,
		kubeconfig:    kubeconfig,
		context:       context,
		clusterConfig: clusterConfig,
	}
}

// McConfigAdded should be called when a a Multi-cluster config has been added
func (cm *ConfigsManagement) McConfigAdded(config model.Config) {
	nsClient, err := makeK8sServicesClient(cm.kubeconfig, cm.context, config.Namespace)
	if err != nil {
		log.Errorf("Failed to create k8s services client for NS %s: %v", config.Namespace, err)
		return
	}
	svcList, err := nsClient.List(metav1.ListOptions{})
	if err != nil {
		log.Errora(err)
	}
	reconciler := reconcile.NewReconciler(cm.istioStore, svcList.Items, cm.clusterConfig)
	changes, err := reconciler.AddMulticlusterConfig(config)
	if err != nil {
		log.Errora(err)
		return
	}
	storeIstioConfigs(cm.istioStore, changes)
	storeK8sConfigs(changes.Kubernetes, nsClient)
}

// McConfigDeleted should be called when a a Multi-cluster config has been deleted
func (cm *ConfigsManagement) McConfigDeleted(config model.Config) {
	nsClient, err := makeK8sServicesClient(cm.kubeconfig, cm.context, config.Namespace)
	if err != nil {
		log.Errorf("Failed to create k8s services client for NS %s: %v", config.Namespace, err)
		return
	}
	svcList, err := nsClient.List(metav1.ListOptions{})
	if err != nil {
		log.Errora(err)
	}
	reconciler := reconcile.NewReconciler(cm.istioStore, svcList.Items, cm.clusterConfig)
	changes, err := reconciler.DeleteMulticlusterConfig(config)
	if err != nil {
		log.Errora(err)
		return
	}
	storeIstioConfigs(cm.istioStore, changes)
	storeK8sConfigs(changes.Kubernetes, nsClient)
}

// McConfigModified should be called when a a Multi-cluster config has been modified
func (cm *ConfigsManagement) McConfigModified(config model.Config) {
	nsClient, err := makeK8sServicesClient(cm.kubeconfig, cm.context, config.Namespace)
	if err != nil {
		log.Errorf("Failed to create k8s services client for NS %s: %v", config.Namespace, err)
		return
	}
	svcList, err := nsClient.List(metav1.ListOptions{})
	if err != nil {
		log.Errora(err)
	}
	reconciler := reconcile.NewReconciler(cm.istioStore, svcList.Items, cm.clusterConfig)
	changes, err := reconciler.ModifyMulticlusterConfig(config)
	if err != nil {
		log.Errora(err)
		return
	}
	storeIstioConfigs(cm.istioStore, changes)
	storeK8sConfigs(changes.Kubernetes, nsClient)
}

// StoreIstioConfigs updates the provided ConfigStore with the created, updated and deleted configs
func storeIstioConfigs(store model.ConfigStore, changes *reconcile.ConfigChanges) {
	if changes == nil {
		return
	}
	if len(changes.Modifications) > 0 {
		log.Debugf("Istio configs updated: %d", len(changes.Modifications))
		for _, cfg := range changes.Modifications {
			_, err := store.Update(cfg)
			if err != nil {
				log.Warnf("\tType:%s\tName: %s.%s [Error: %v]", cfg.Type, cfg.Name, cfg.Namespace, err)
				continue
			}
			log.Debugf("\tType:%s\tName: %s.%s [Updated]", cfg.Type, cfg.Name, cfg.Namespace)
		}
	}
	if len(changes.Additions) > 0 {
		log.Debugf("Istio configs created: %d", len(changes.Additions))
		for _, cfg := range changes.Additions {
			_, err := store.Create(cfg)
			if err != nil {
				log.Warnf("\tType:%s\tName: %s.%s [Error: %v]", cfg.Type, cfg.Name, cfg.Namespace, err)
				continue
			}
			log.Debugf("\tType:%s\tName: %s.%s [Created]", cfg.Type, cfg.Name, cfg.Namespace)
		}
	}
	if len(changes.Deletions) > 0 {
		log.Debugf("Istio configs deleted: %d", len(changes.Deletions))
		for _, cfg := range changes.Deletions {
			err := store.Delete(cfg.Type, cfg.Name, cfg.Namespace)
			if err != nil {
				log.Warnf("\tType:%s\tName: %s.%s [Error: %v]", cfg.Type, cfg.Name, cfg.Namespace, err)
				continue
			}
			log.Debugf("\tType:%s\tName: %s.%s [Deleted]", cfg.Type, cfg.Name, cfg.Namespace)
		}
	}
}

func storeK8sConfigs(changes *reconcile.KubernetesChanges, k8sSvcClient corev1.ServiceInterface) {
	if changes == nil {
		return
	}
	if len(changes.Modifications) > 0 {
		log.Debugf("Kubernetes services updated: %d", len(changes.Modifications))
		for _, cfg := range changes.Modifications {
			_, err := k8sSvcClient.Update(&cfg)
			if err != nil {
				log.Warnf("\tService Name: %s.%s [Error: %v]", cfg.Name, cfg.Namespace, err)
				continue
			}
			log.Debugf("\tService Name: %s.%s [Updated]", cfg.Name, cfg.Namespace)
		}
	}
	if len(changes.Additions) > 0 {
		log.Debugf("Kubernetes services created: %d", len(changes.Additions))
		for _, cfg := range changes.Additions {
			_, err := k8sSvcClient.Create(&cfg)
			if err != nil {
				log.Warnf("\tService Name: %s.%s [Error: %v]", cfg.Name, cfg.Namespace, err)
				continue
			}
			log.Debugf("\tService Name: %s.%s [Created]", cfg.Name, cfg.Namespace)
		}
	}
	if len(changes.Deletions) > 0 {
		log.Debugf("Kubernetes services deleted: %d", len(changes.Deletions))
		for _, cfg := range changes.Deletions {
			err := k8sSvcClient.Delete(cfg.Name, &metav1.DeleteOptions{})
			if err != nil {
				log.Warnf("\tService Name: %s.%s [Error: %v]", cfg.Name, cfg.Namespace, err)
				continue
			}
			log.Debugf("\tService Name: %s.%s [Deleted]", cfg.Name, cfg.Namespace)
		}
	}
}

func makeK8sServicesClient(kubeconfig, context, namespace string) (corev1.ServiceInterface, error) {
	config, err := kubecfg.BuildClientConfig(kubeconfig, context)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return clientset.CoreV1().Services(namespace), nil
}
