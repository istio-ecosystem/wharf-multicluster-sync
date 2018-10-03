# Multicluster Istio

This project attempts to create a proof-of-concept for Multicluster Istio configuration
based on _ServiceExpositionPolicies_ and _RemoteServiceBindings_.  For more
details see the document _Multi-cluster Networking for Independent Clusters_
by Etai Lev Ran.

The design allows any number of clusters.  Clusters may contribute, consume, or both contribute
and consume services.  A cluster may be a "donor" or "acceptor" or both.  A cluster may
offer services that are used by multiple clusters and a service may consume the same service from multiple
clusters.

## Demo

You'll need two clusters with public IP addresses.

Follow [these instructions](docs/install/README.md) to set up your clusters to share a Root CA
and install experimental Multicluster Agents.

Then try the [Multicluster bookinfo tutorial](docs/tutorial/bookinfo/README.md).

## User Experience

This design assumes services are exposed and consumed by separate clusters, possibly managed by independent parties. The goal is to allow one cluster to consume services explicitly exposed by the other, maintaining clear service boundaries, encapsulation and authority of control. We follow a similar design flow to the way Kubernetes handles Persistent Volume (PV) and Persistent Volume Claims (PVC).

For the sake of clarity, we denote the cluster exposing the service as the server  cluster and the cluster consuming the service as the client cluster. It should be noted that this defines a relation with respect to a specific service, and any cluster may serve as both server and client concurrently, with respect to other services.

In order to initiate service exposition from one cluster to another under this design:

### Prerequisites
- Both clusters should have unique names that are defined and known to the clusters’ operators. This may leverage a cluster registry, as proposed by Kubernetes Federation v2, or any other registry or agreement, including out of band mechanism;
- Both clusters must share a common root of trust to allow the donor cluster to confirm accesses originating from the acceptor cluster, while denying other access requests;
### Server Cluster
- Defines a Service Exposition Policy (the equivalent of a PV) referencing a VirtualService, a list of allowed consumers in the form of client cluster names (and possibly a services), and an optional alias under which it should be exposed. Since the policy specifies a VirtualService, it can expose the service and associated behaviors (e.g., timeout, subsets, etc.);
- Internally, the policy configures an Istio ingress gateway to expose the service (or subset defined) optionally using the alias name. This is described in the Design section below. Together, the policy and donor configuration create an externally accessible reference to the VirtualService plus the configuration needed to accept connections from configured remote clusters.

### Client cluster
- Establishes a peering relation with the server cluster so it can react to service exposition policy events. This too may leverage a central cluster registry or be configured locally in the client cluster.
- Defines a Remote Service Binding (the equivalent of the PVC) naming a remote exposed service (both cluster and service names are required), along with any needed client cluster metadata (e.g., namespace where cluster should be exposed). Note that multiple service bindings may exist concurrently in the client cluster.
- Internally, the binding will be used to configure Istio (and Kubernetes) to allow consumption of the remote service securely.
