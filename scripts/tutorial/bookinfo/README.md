
# Prerequisite

This demo assumes you have configured Istio with multicluster agents.  Follow the
instructions [here](../../scripts/install/README.md)

# Demo

First, install Bookinfo with _details_ and _productpage_ on cluster1.
The _reviews_ (version:v1) will be on cluster2

```
kubectl --context $CLUSTER1 apply -f bookinfo-norating-noreviews.yaml
kubectl --context $CLUSTER1 apply -f bookinfo-gateway.yaml
kubectl --context $CLUSTER2 apply -f bookinfo-reviews-v1.yaml
```

![alt text](/raw/master/scripts/tutorial/bookinfo/bookinfo-unconnected.png "Unconnected Bookinfo")

## Before the services are connected

Follow the instructions at https://istio.io/docs/examples/bookinfo/#determining-the-ingress-ip-and-port to find Bookinfo.

Browse to http://${GATEWAY_URL}/productpage to verify that bookinfo is connected and that
details are present but not reviews.

TODO PICTURE OF BOOKINFO WITHOUT REVIEWS

## Expose reviews-v1 on cluster2

We will create a Service Exposition Policy to expose reviews-v1 on cluster2 to services on cluster1.

```
kubectl --context $CLUSTER2 apply -f reviews-exposure.yaml
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

You should see them.

## Inspect the binding on cluster1

Exposing on cluster2 caused a _RemoteServiceBinding_ to be created on cluster1.

```
kubectl --context $CLUSTER1 get remoteservicebindings
```

Because the agent is running in `live` mode the client-side Istio configuration was created automatically.

```
kubectl --context $CLUSTER1 get service,destinationrule,serviceentry
```

## Verify that the reviews is present in the UI

Reload http://${GATEWAY_URL}/productpage in the browser.  Verify that reviews are present.

## Deploying and Exposing the Ratings microservice

We will now deploy a _ratings_ service on cluster1 and expose it.

```
kubectl --context $CLUSTER1 apply -f bookinfo-ratings.yaml
kubectl --context $CLUSTER1 apply -f ratings-exposure.yaml
```

TODO Only expose ratings to cluster2

As before, exposing ratings created configuration on peer cluster2:

```
kubectl --context $CLUSTER2 get remoteservicebindings,gateways,virtualservices,destinationrules
```

## Deploying a new version of reviews that will consume ratings

