
# Setting your environment to run the demo

To run this demo you will need three Kubernetes clusters.  They should all be available
as contexts in the same Kubernetes configuration.  For example, you should be able to
run `kubectl --context <ctx1> get pods`, for three contexts. 

If you are using $KUBECONFIG you can export multiple config files for this behavior, e.g.

```
export KUBECONFIG=$KUBECONFIG1:$KUBECONFIG2:$KUBECONFIG3
```

To setup the test environments run the following, replacing _ctx-ca_, _ctx1_, and _ctx2_ with your cluster names:

```
source ./demo_context.sh ctx-ca ctx1 ctx2
```

# Configuring Istio to use a common Citadel 

The _install_citadel.sh_ script will configure $CLUSTER1 and $CLUSTER2 to use Citadel on $ROOTCA_NAME.

```
./install_citadel.sh
```

# Run the demo agents on the Istio control planes

First, we configure the agents.  Next, deploy them.  For this demo we will make $CLUSTER1
a client of $CLUSTER2.

```
./deploy_cluster.sh $CLUSTER1 $CLUSTER2
```

