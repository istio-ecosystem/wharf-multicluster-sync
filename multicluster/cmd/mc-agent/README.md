# Multi-Cluster Agent

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
