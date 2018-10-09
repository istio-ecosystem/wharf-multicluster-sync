## Types Generator

The generator tool and the template has been taken from Istio's Pilot.
The tool will generate the `types.go` used by the controller according to the config model defined in `multicluster/model/config.go`.

To re-generate the `types.go` (after modifying the `config.go`) execute the following command from the root folder:
```command
go run multicluster/tools/generate_config_crd_types.go -output multicluster/pkg/config/kube/crd/types.go
```

When/if this code will be part of Istio then the Pilot tool can be used instead of this one.

## Private Multi-cluster Gateways

Private gateways are used for the communication between the connected clusters for both control and data planes. Those gateways are not the default Istio gateways therefore should be added to the involved clusters **after** installing Istio.

### Install with Helm
In this folder you can find a Helm values file, `values-istio-mc-gateways.yaml`, that can be used to install the private multi-cluster gateways in addition to existing Istio default gateways.

To install, execute the following command from your Istio folder:
```command
helm install install/kubernetes/helm/istio --namespace=istio-system --name=istio --values=<PATH_TO_HERE>/install/values_istio_mc_gateways.yaml --kube-contex=<CLUSTER_CONTEXT>
```

Where `<PATH_TO_HERE>` should be replaced with an abstract path to this install folder and the `<CLUSTER_CONTEXT>` is the Kubeconfig context for the cluster where you want these gateways to be added.

### Install without Helm
If you encounter problems installing with Helm or if Helm/Tiller are not available for your cluster, you can also install the gateways by executing the following command:

```command
kubectl apply -f mc_gateways.yaml --context=<CLUSTER_CONTEXT>
```

where the `<CLUSTER_CONTEXT>` is the Kubeconfig context for the cluster where you want these gateways to be added.

> This YAML file is the actually the output of executing `helm template` with the values file of previous section. To update it, modify the values file and re-generate it.
