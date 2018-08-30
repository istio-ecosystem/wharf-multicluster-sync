# MultiCluster Demos 

## Basic Scenarios with the Bookinfo sample application (3 clusters)

## Background

Three peer organizations, _org1_, _org2_, and _org3_ control their own individual clusters, referred to as _cluster1_, _cluster2_, and _cluster3_, respectively.

### The inception of the _Bookinfo_ application

In _org1_, a developer team is resposible for creating the _Bookinfo_ application. They start by creating 2 microservices, namely, _productpage_ and _details_. They learn about a microservice made available by a team from _org2_ and decide to use it. Thus, the first version of _bookinfo_ is finished, which uses the version 1 of the _reviews_ service (_reviews-v1_) made available by _org2_. 

**Demo steps:**

1. Start with services _productpage_ and _details_ running on _cluster1_.
2. Start with service _reviews-v1_ running on _cluster2_.
3. Expose _reviews-v1_ on _cluster2_.
4. Bind _productpage_ (on _cluster1_) to _reviews-v1_ (on _cluster2_).
5. Show the _Bookinfo_ running with _productpage_, _details_, and _reviews-v1_.

### New version of _reviews_ is available

In _org2_, the dev team of _reviews_ learns about the _ratings_ service made available by _org3_. So, they decide to build a new version of their service (_reviews-v2_) which takes advantage of the _ratings_ service provided by _org3_. The new version of reviews is then made available by _org3_.

**Demo steps:**

1. Make sure _ratings_ is running on _cluster3_.
2. Expose _reviews-2_ on _cluster2_.
3. Using the _Bookinfo_ application, show that by defaul traffic is split between _reviews-v1_ and _reviews-v2_ in a round-robin fashion. Note that the new version of a service available on _cluster2_ was automatically picked up by _cluster1_.

### Dev team of _producpage_ decides to use _reviews-v2_ only

In _org1_, the developers of _productpage_, aware of _reviews-v2_, decide to stop using _reviews-v1_.

**Demo steps:**

1. From _cluster1_, create a destination rule to use only _reviews-v2_.
2. Show the result by using the _Bookinfo_ application.
