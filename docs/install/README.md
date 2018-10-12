
# Setting your environment to run the demo

To run this demo you will need two Kubernetes clusters.  They should be available
as contexts in the same Kubernetes configuration.  For example, you should be able to
run `kubectl --context <ctx1> get pods`, for each context. 

If you are using $KUBECONFIG you can export multiple config files for this behavior, e.g.

```
export KUBECONFIG=$KUBECONFIG1:$KUBECONFIG2
```

To setup the test environments run the following.  Use your Kubernetes cluster names as the contexts.  (_context-ca_ may match one of the other contexts.)

```
source ./demo_context.sh <context-ca> <context1> <context2>
```

# Configure Istio Citadel to use a shared upstream Certificate Authority 

The _install_citadel.sh_ script will configure $CLUSTER1 and $CLUSTER2 to use Citadel on $ROOTCA_NAME.

```
./install_citadel.sh
```

![Shared Certificate Authority](shared-ca.png?raw=true "Shared Certificate Authority")


# Run the Multi-Cluster agents on demo clusters

In this demo we have Cluster 1 watching exposed services on Cluster 2.
For this purpose we need to deploy the MC agent on both clusters and configure Cluster 1's agent
to peer with Cluster 2's agent.

We then configure and deploy the agent on `$CLUSTER1` and ask it to peer with `$CLUSTER2` (the 2nd argument).

```
./deploy_cluster.sh cluster1=$CLUSTER1 cluster2=$CLUSTER2
./deploy_cluster.sh cluster2=$CLUSTER2 cluster1=$CLUSTER1
```

The script deploys the agent and configures it with the Istio Gateway of their peer.

# Tutorials

Now that the Multicluster control plane has been configured we can run demos

- [Book Info](../tutorial/bookinfo/README.md)

# Cleanup

No instructions are provided for removing the root CA sharing.  If you wish to return to the
original you should reinstall Istio on your clusters.

To cleanup the multicluster agents,


```
kubectl --context $CLUSTER1 delete -f deploy.yaml 
kubectl --context $CLUSTER2 delete -f deploy.yaml 
kubectl --context $CLUSTER1 delete cm mc-configuration -n istio-system
kubectl --context $CLUSTER2 delete cm mc-configuration -n istio-system
kubectl --context $CLUSTER1 -n istio-system patch service istio-ingressgateway --type=json --patch='[{"op": "test", "path": "/spec/ports/0/port", "value": 31444}, {"op": "remove", "path": "/spec/ports/0"}]' || true
kubectl --context $CLUSTER2 -n istio-system patch service istio-ingressgateway --type=json --patch='[{"op": "test", "path": "/spec/ports/0/port", "value": 31444}, {"op": "remove", "path": "/spec/ports/0"}]' || true
```
