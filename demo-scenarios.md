# MultiCluster Demos

## Basic scenarios with the _Bookinfo_ sample application (3 clusters)

## Background

Three peer organizations, _org1_, _org2_, and _org3_ control their own individual clusters, referred to as _cluster1_, _cluster2_, and _cluster3_, respectively.

### The inception of the _Bookinfo_ application

In _org1_, a developer team is responsible for creating the _Bookinfo_ application. They start by creating 2 microservices, namely, _productpage_ and _details_. They learn about a microservice made available by a team from _org2_ and decide to use it. Thus, the first version of _bookinfo_ is finished, which uses the version 1 of the _reviews_ service (_reviews-v1_) made available by _org2_.

**Demo steps:**

1. Start with services _productpage_ and _details_ running on _cluster1_.
2. Start with service _reviews-v1_ running on _cluster2_.
3. Expose _reviews-v1_ on _cluster2_.
4. From _cluster1_, show all available services available on _cluster2_. (Note that this assumes we can do this either manually, as suggested by this step, or automatically. Other parts of the demo (see below) will demonstrate the automatic configuration.)
5. Bind _productpage_ (on _cluster1_) to _reviews-v1_ (on _cluster2_). (Again, this assumes we can do it manually.)
6. Show _Bookinfo_ running with _productpage_, _details_, and _reviews-v1_.

### New version of _reviews_ is available

In _org2_, the dev team of _reviews_ learns about the _ratings_ service made available by _org3_. So, they decide to build a new version of their service (_reviews-v2_) which takes advantage of the _ratings_ service provided by _org3_. The new version of reviews is then made available by _org3_.

**Demo steps:**

1. Make sure _ratings_ is running on _cluster3_.
2. Expose _ratings_ on cluster3.
3. Expose _reviews-v2_ on _cluster2_.
4. Using the _Bookinfo_ application, show that by default traffic is split between _reviews-v1_ and _reviews-v2_ in a round-robin fashion. Note that the new version of a service available on _cluster2_ was automatically picked up by _cluster1_, and that a service exposed by _cluster3_ was automatically picked up by _cluster2_.

### Dev team of _productpage_ decides to use _reviews-v2_ only

In _org1_, the developers of _productpage_, aware of _reviews-v2_, decide to stop using _reviews-v1_.

**Demo steps:**

1. From _cluster1_, create a destination rule to use only _reviews-v2_ on _cluster2_.
2. Show the result by using the _Bookinfo_ application.
