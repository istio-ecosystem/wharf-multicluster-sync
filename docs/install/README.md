
# Prerequisities
This demo assumes that you have Istio (with auth enabled) installed on your clusters.

# Setting your environment to run the demo

To run this demo you will need two Kubernetes clusters. They should be available
as contexts in the same Kubernetes configuration.  For example, you should be able to
run `kubectl --context <ctx> get pods`, for each context. 

If you are using $KUBECONFIG you can export multiple config files for this behavior, e.g.

```sh
export KUBECONFIG=$KUBECONFIG1:$KUBECONFIG2
```

To setup the necessary environment variables run the following.  Use your Kubernetes cluster names as the contexts.  (_context-ca_ may match one of the other contexts.)

```sh
source ./demo_context.sh <context-ca> <context1> <context2>
```

# Configure Istio Citadel to use a shared upstream Certificate Authority

The _install_citadel.sh_ script will configure _$CLUSTER1_ and _$CLUSTER2_ to use Citadel on _$ROOTCA_NAME_.

```sh
./install_citadel.sh
```

![Shared Certificate Authority](shared-ca.png?raw=true "Shared Certificate Authority")


# Run the Multi-Cluster agents on demo clusters

In this demo we have Cluster 1 watching exposed services on Cluster 2.
For this purpose we need to deploy the MC agent on both clusters and configure Cluster 1's agent to peer with Cluster 2's agent.

We then configure and deploy the agent on `$CLUSTER1` and ask it to peer with `$CLUSTER2` (the 2nd argument).

```sh
./deploy_cluster.sh cluster1=$CLUSTER1 cluster2=$CLUSTER2
./deploy_cluster.sh cluster2=$CLUSTER2 cluster1=$CLUSTER1
```

The script deploys the agent and configures it with the Istio Gateway of their peer.

# Tutorials

Now that the Multicluster control plane has been configured we can run demos

- [Book Info](../tutorial/bookinfo/README.md)
- [Book Info via Command Line](../tutorial/command-line/README.md)

# Cleanup

No instructions are provided for removing the root CA sharing.  If you wish to return to the
original you should reinstall Istio on your clusters.

To cleanup the multicluster agents:

```sh
./cleanup_cluster.sh $CLUSTER1
./cleanup_cluster.sh $CLUSTER2
```
