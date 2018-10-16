# Prerequisite

This demo assumes you have configured Istio with multicluster agents.  Follow the
instructions [here](/istio-ecosystem/wharf-multicluster-sync/tree/master/docs/install/README.md)

# Building the command line

To build the command line,

```
cd multicluster
make cli
cp build/mc-tool /usr/local/bin
```

Note: Requires Go.

Create the configurations

```
./gen_cluster_config.sh cluster1=$CLUSTER1 cluster2=$CLUSTER2
./gen_cluster_config.sh cluster2=$CLUSTER2 cluster1=$CLUSTER1
```

Configure the Custom Resource Definition and the IngressGateways

```
kubectl --context $CLUSTER1 apply crddefs.yaml
kubectl --context $CLUSTER1 patch service istio-ingressgateway -n istio-system --type=json --patch='[{"op": "add", "path": "/spec/ports/0", "value": {"name": "tls-intermesh", "port": 31444, "nodePort": 31444, "targetPort": 31444}}]'
kubectl --context $CLUSTER2 apply crddefs.yaml
kubectl --context $CLUSTER2 patch service istio-ingressgateway -n istio-system --type=json --patch='[{"op": "add", "path": "/spec/ports/0", "value": {"name": "tls-intermesh", "port": 31444, "nodePort": 31444, "targetPort": 31444}}]'
```


# Demo

First, install Bookinfo with _details_ and _productpage_ on cluster1.
The _reviews_ (version:v1) will be on cluster2

```
kubectl --context $CLUSTER1 apply -f bookinfo-norating-noreviews.yaml
kubectl --context $CLUSTER1 apply -f bookinfo-gateway.yaml
kubectl --context $CLUSTER2 apply -f bookinfo-reviews-v1.yaml
```

![Unconnected Bookinfo](../bookinfo/bookinfo-unconnected.png?raw=true "Unconnected Bookinfo")

## Before the services are connected

Follow the instructions at https://istio.io/docs/examples/bookinfo/#determining-the-ingress-ip-and-port to find Bookinfo.

Browse to http://${GATEWAY_URL}/productpage to verify that bookinfo is connected and that
details are present but not reviews.

![Unconnected UI](../bookinfo/ui-unconnected.png?raw=true "Unconnected UI")

## Expose reviews-v1 on cluster2

We will create a Service Exposition Policy to expose reviews-v1 on cluster2 to services on cluster1.

```
mc-tool --filename ../bookinfo/reviews-exposure.yaml --mc-conf-filename cluster2-conf.yaml |kubectl --context $CLUSTER2 apply -f -
```

The policy we are applying looks like

```
## Expose the "reviews" service
apiVersion: multicluster.istio.io/v1alpha1
kind: ServiceExpositionPolicy
metadata:
  name: reviews
spec:
  exposed:
  - name: reviews
    port: 9080
```

It just says to expose _reviews:9080_ to all clusters, all namespaces, within our Root CA.

Upon running the above command the agent created Istio configuration on $CLUSTER2.  Verify that
the configuration was created

```
kubectl --context $CLUSTER2 get gateways,virtualservices,destinationrules
```

## Bind cluster1 to reviews on cluster2

```
mc-tool --genbinding cluster1 --filename ../bookinfo/reviews-exposure.yaml --mc-conf-filename cluster2-conf.yaml | kubectl --context $CLUSTER1 apply -f -
```

Upon running the above command the agent created Istio configuration on $CLUSTER2.  Verify that
the configuration was created

```
kubectl --context $CLUSTER2 get gateways,virtualservices,destinationrules
```

You should see a gateway, virtual service, and destination rule for reviews.  The topology
is now

![Bookinfo with reviews v1](../bookinfo/bookinfo-reviews-v1.png?raw=true "Bookinfo with reviews v1")

## Inspect the binding on cluster1

Exposing on cluster2 caused a _RemoteServiceBinding_ to be created on cluster1.

```
kubectl --context $CLUSTER1 get remoteservicebindings
```

If the binding is in `live` mode the client-side Istio configuration will be created automatically.

```
kubectl --context $CLUSTER1 get service,destinationrule,serviceentry
```

## Verify that the reviews is present in the UI

![Connected UI](../bookinfo/ui-connected.png?raw=true "Connected UI")

Reload http://${GATEWAY_URL}/productpage in the browser.  Verify that reviews are present.

# Cleanup

To remove the demo artifacts, execute the following:

```
mc-tool --filename ../bookinfo/reviews-exposure.yaml --mc-conf-filename cluster2-conf.yaml | kubectl --context $CLUSTER2 delete -f -
mc-tool --genbinding cluster1 --filename ../bookinfo/reviews-exposure.yaml --mc-conf-filename cluster2-conf.yaml | kubectl --context $CLUSTER1 delete -f -
kubectl --context $CLUSTER1 delete service reviews
kubectl --context $CLUSTER1 delete -f bookinfo-norating-noreviews.yaml
kubectl --context $CLUSTER1 delete -f bookinfo-gateway.yaml
kubectl --context $CLUSTER2 delete -f bookinfo-reviews-v1.yaml
```


