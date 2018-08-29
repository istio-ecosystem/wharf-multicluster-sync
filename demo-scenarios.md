# MultiCluster Demos 

## Basic Scenarios with the Bookinfo sample application

## Proposed Alternative
This proposed version of the demo (a) demonstrates the same things as the original, (b) avoids the need to reset between scenarios, and (c) allows us to explicitly demonstrate the work needed to expose a service and for a client to bind to it. The original demo scenario is maintained below if we wish to use it.

### _Scenario 1_: Using a service (_reviews_) on a remote cluster

1. Start with services _productpage_ and _details_ running on _cluster1_ 
2. Start with services _reviews-v1_ and _reviews-v2_ running on _cluster2_.
3. Note that the service _ratings_ is not running.
4. Demonstrate the _bookinfo_ application. 

Traffic will, by default, be sent to instances of both _reviews-v1_ and _reviews-v2_. The traffic to _reviews-v1_ will work without error. That to _reviews-v2_ should display an error since the service _ratings_ is not available.

### _Scenario 2_: Exposing a service on a remote cluster

1. Deploy service _ratings_ to _cluster3_
2. Expose service _ratings_ on _cluster3_
3. Bind service _reviews-v2_ (on _cluster2_) to the remote service _ratings_ (on _cluster3_)
4. Demonstrate the _bookinfo_ application

Traffic will, by default, be sent to instances of both _reviews-v1_ and _reviews-v2_. The traffic to both should now work error free.

### _Scenario 3_: Demonstate service owners ability to split traffic to multiple versions

1. Define a `DestinationRule` to route traffic from _productpage_ to _reviews-v2_.
2. Demonstrate with _bookinfo_ application.

### _Scenario 3_: (3 clusters): TBD


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

