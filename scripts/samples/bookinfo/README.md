
After setting up 4 clusters, one for the root CA and the other three clusters
(named cluster_a, cluster_b, and cluster_c) for Bookinfo services
follow these steps:


# Demo

1. Basic Bookinfo with details and productpage on cluster_a and reviews version v1 on cluster_b.

```
 ./create_demo_1.sh
```

2. Adding reviews v2 and v3 to cluster_b and ratings to cluster_c.

```
 ./create_demo_2.sh
```

3. Selecting a subset of reviews to be used.

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

run config_dump.sh and inspect the output in config_dump.json. It contains the info on all rekated resources.