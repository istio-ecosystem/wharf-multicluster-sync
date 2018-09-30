#!/bin/bash
set -e

source ~/Sandbox/kube_context.sh

SCRIPTDIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )
ISTIODIR="/Users/mb/Repos/istio-1.0.2"

# To make sure we can ccess all clusters
kubectl --context ${CLUSTER1_NAME} get nodes
kubectl --context ${CLUSTER2_NAME} get nodes
kubectl --context ${CLUSTER3_NAME} get nodes
kubectl --context ${ROOTCA_NAME} get nodes

# Installing Cluster 1 
for CLUSTER in ${CLUSTER1_NAME} ${CLUSTER2_NAME} ${CLUSTER3_NAME}
do

   echo "on cluster" ${CLUSTER} " ..................................................................."
   # Installing Istio
   kubectl --context ${CLUSTER} create namespace istio-system || true
   kubectl --context ${CLUSTER} apply -f ${ISTIODIR}/install/kubernetes/helm/istio/templates/crds.yaml
   kubectl --context ${CLUSTER1_NAME} apply -f ${ISTIODIR}/install/kubernetes/istio-demo-auth.yaml
done
