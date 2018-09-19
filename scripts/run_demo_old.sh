#!/bin/bash
set -e

source ~/Sandbox/kube_context.sh

SCRIPTDIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )
ISTIODIR="/Users/mb/Repos/istio-1.0.2"

# To make sure we can ccess all clusters
kubectl --context ${CLUSTER1_NAME} get nodes
kubectl --context ${CLUSTER2_NAME} get nodes
kubectl --context ${ROOTCA_NAME} get nodes

# Install the ROOT CA
kubectl --context ${ROOTCA_NAME} apply -f istio-citadel-standalone.yaml
kubectl --context ${ROOTCA_NAME} -n istio-system create serviceaccount istio-citadel-service-account-${CLUSTER1_ID} 
kubectl --context ${ROOTCA_NAME} -n istio-system create serviceaccount istio-citadel-service-account-${CLUSTER2_ID}
rootca_host=`kubectl --context ${ROOTCA_NAME} get service standalone-citadel -n istio-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}'`

# Installing Cluster 1 
echo "on cluster 1 ..................................................................."
# Installing Istio
kubectl --context ${CLUSTER1_NAME} create namespace istio-system || true
kubectl --context ${CLUSTER1_NAME} apply -f ${ISTIODIR}/install/kubernetes/helm/istio/templates/crds.yaml
${SCRIPTDIR}/provision_cluster_int_ca.sh $ROOTCA_NAME $CLUSTER1_NAME $CLUSTER1_ID
kubectl --context ${CLUSTER1_NAME} apply -f ${ISTIODIR}/install/kubernetes/istio-demo-auth.yaml
sleep 5
# Installing new citadel
kubectl --context ${CLUSTER1_NAME} delete  deployment  -n istio-system   istio-citadel
sed -e "s/__CLUSTERNAME__/${CLUSTER1_ID}/g;s/__ROOTCA_HOST__/${rootca_host}/g" istio-citadel-new.yaml | kubectl --context ${CLUSTER1_NAME} apply -f -

# Installing Cluster 2
echo "on cluster 2 ..................................................................."
# Installing Istio
kubectl --context ${CLUSTER2_NAME} create namespace istio-system || true
kubectl --context ${CLUSTER2_NAME} apply -f ${ISTIODIR}/install/kubernetes/helm/istio/templates/crds.yaml
${SCRIPTDIR}/provision_cluster_int_ca.sh $ROOTCA_NAME $CLUSTER2_NAME $CLUSTER2_ID
kubectl --context ${CLUSTER2_NAME} apply -f ${ISTIODIR}/install/kubernetes/istio-demo-auth.yaml
sleep 5
# Installing new citadel
kubectl --context ${CLUSTER2_NAME} delete  deployment  -n istio-system   istio-citadel
sed -e "s/__CLUSTERNAME__/${CLUSTER2_ID}/g;s/__ROOTCA_HOST__/${rootca_host}/g" istio-citadel-new.yaml | kubectl --context ${CLUSTER2_NAME} apply -f -

echo "That is it for now."