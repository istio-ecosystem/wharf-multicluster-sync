# Istio Multi-Cluster Installation

## Setting up a cluster

The Multi-Cluster agent is a service running within the cluster and both serves requests from agents on other clusters and watches peered agents.

The configuration for the agent is a ConfigMap to be deployed prior to deploying the agent. The following is a sample ConfigMap:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: mc-configuration
  namespace: default
  labels:
    istio: multi-cluster-agent
data:
  config.yaml: |
      ID: cluster-a
      GatewayIP: 159.8.183.116
      GatewayPort: 80
      AgentPort: 8999
      TrustedPeers: []
      WatchedPeers:
      - ID: cluster-b
        GatewayIP: 159.122.217.84
        GatewayPort: 80
        AgentIP: 159.122.217.82
        AgentPort: 80 
        ConnectionMode: live
```
In this configuration the agent is running on cluster `cluster-a` and watches for changes on a remote cluster `cluster-b`. No peers are expected to peer with this one because the `TrustedPeers` list is empty.

Once the ConfigMap has been configured with the relevant values, deploy it to your cluster. E.g.:
```sh
kubectl create -f cluster-a.yaml
```

Once configuration is set you are ready to deploy the agent itself that will read the configuration just deployed and start listening and watching other peers. To deploy run:
```sh
kubectl create -f deploy.yaml
```

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
