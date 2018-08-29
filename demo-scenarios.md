# MultiCluster Demos 

## Basic Scenarios with the Bookinfo sample application

### _Scenario 1_ (2 clusters): Using a service available in a remote cluster

1. Start with the services _productpage_, _details_, and reviews-v1_ running on _cluster1_.
2. Start with the _ratings_ service running on _cluster2_.
3. Now, the owners of the _reviews_ service want to upgrade to version 2 (_reviews-v2_). In doing so, they will point _reviews-v2_
to the _ratings_ service already available on _cluster2_. Result: _reviews_ running on _cluster1_ will communicate 
with _ratings_ running on _cluster2_.


### _Scenario 2_ (2 clusters): Traffic splitting

1. Start with the services _productpage_ and _details_ running on _cluster1_.
2. Start with the services _reviews_ (all 3 versions) and _ratings_ running on _cluster2_.
3. Show that we can split the traffic across all versions of _reviews_. Result: _productpage_ running on _cluster1_
will talk to all versions of _reviews_ running on _cluster2_.

### _Scenario 3_ (3 clusters): TBD

