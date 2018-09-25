
After setting up 4 clusters, one for the root CA and the other three clusters
(named cluster_a, cluster_b, and cluster_c) for Bookinfo services
follow these steps:


# Demo

1. Basic Bookinfo with details and productpage on cluster_a and reviews version v1 on cluster_b.

```
 ./setup_demo_1.sh
```

2. Determine the IP address and Port.  Use `kubectl --context $CLUSTER1 get services -n istio-system`.
The IP address is the public address of istio-ingressgateway.  If it does not have an address use
`bx cs workers $CLUSTER1` and use the public address there.  The port is the port from istio-ingressgateway,
typically 31380.

Use the browser to go to IP:31380/productpage.  Show that Productpage does not have access to reviews.

Do `kubectl --context $CLUSTER2 create  -f ./reviews-exposure.yaml`

3. Adding reviews v2 and v3 to cluster_b and ratings to cluster_c.

```
kubectl --context $CLUSTER3 apply  -f bookinfo-ratings.yaml
kubectl --context ${CLUSTER3} apply -f ratings-exposure.yaml
 kubectl --context ${CLUSTER2} apply  -f bookinfo-reviews-v2.yaml
```

4. Selecting a subset of reviews to be used.

```
 ./ceate_demo_3.sh
```

# Cleanup

```
./delete_demo_3.sh
./delete_demo_2.sh
./delete_demo_1.sh
```

# Debug

run `config_dump.sh` and inspect the output in _config_dump.json_. It contains the info on all related resources.