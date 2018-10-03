
# Setting your environment to run the demo

To run this demo you will need three Kubernetes clusters.  They should all be available
as contexts in the same Kubernetes configuration.  For example, you should be able to
run `kubectl --context <ctx1> get pods`, for three contexts. 

If you are using $KUBECONFIG you can export multiple config files for this behavior, e.g.

```
export KUBECONFIG=$KUBECONFIG1:$KUBECONFIG2:$KUBECONFIG3
```

To setup the test environments run the following.  Use your Kubernetes cluster names as the contexts.  (_context-ca_ may match one of the other contexts.)

```
source ./demo_context.sh <context-ca> <context1> <context2> <context3>
```

# Configuring Istio to use a common Citadel 

The _install_citadel.sh_ script will configure $CLUSTER1 and $CLUSTER2 to use Citadel on $ROOTCA_NAME.

```
./install_citadel.sh
```

# Run the Multi-Cluster agents on demo clusters

(For this demo we are running the multi-cluster control plane outside of Istio.  The intention is to move it into Pilot and Galley.)

In this demo we have Cluster 1 watching exposed services on Cluster 2.
For this purpose we need to deploy the MC agent on both clusters and configure Cluster 1's agent
to peer with Cluster 2's agent.

We then configure and deploy the agent on `$CLUSTER1` and ask it to peer with `$CLUSTER2` (the 2nd argument).  We also peer `$CLUSTER2` with `$CLUSTER3`.

```
./deploy_cluster.sh cluster3=$CLUSTER3
sleep 10
./deploy_cluster.sh cluster2=$CLUSTER2 cluster3=$CLUSTER3
sleep 10
./deploy_cluster.sh cluster1=$CLUSTER1 cluster2=$CLUSTER2
```

The script will get the relevant information (Istio Gateway and MC Agent IP addresses) from and use it in the peer configuration.  (The sleeps are to give the pods a chance to start.)

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
kubectl --context $CLUSTER3 delete -f deploy.yaml
kubectl --context $CLUSTER1 delete cm mc-configuration 
kubectl --context $CLUSTER2 delete cm mc-configuration 
kubectl --context $CLUSTER3 delete cm mc-configuration 
```
