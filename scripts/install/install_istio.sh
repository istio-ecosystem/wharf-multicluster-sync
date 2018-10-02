#!/bin/bash
set -e

source ./kube_context.sh

SCRIPTDIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )
ISTIODIR="/Users/mb/Repos/istio-1.0.2"

# To make sure we can ccess all clusters
kubectl --context ${CLUSTER0} get nodes
kubectl --context ${CLUSTER1} get nodes
kubectl --context ${CLUSTER2} get nodes
kubectl --context ${CLUSTER3} get nodes

# Installing Cluster 1 
for CLUSTER in ${CLUSTER1} ${CLUSTER2} ${CLUSTER3}
do

   echo "on cluster" ${CLUSTER} " ..................................................................."
   # Installing Istio
   kubectl --context ${CLUSTER} create namespace istio-system || true
   kubectl --context ${CLUSTER} apply -f ${ISTIODIR}/install/kubernetes/helm/istio/templates/crds.yaml
   kubectl --context ${CLUSTER} apply -f ${ISTIODIR}/install/kubernetes/istio-demo-auth.yaml
done
