#!/bin/bash
set -e

source ./kube_context.sh

# To make sure we can ccess all clusters
kubectl --context ${CLUSTER1_NAME} get nodes
kubectl --context ${CLUSTER2_NAME} get nodes
kubectl --context ${CLUSTER3_NAME} get nodes
kubectl --context ${ROOTCA_NAME} get nodes

rootca_host=`kubectl --context ${ROOTCA_NAME} get service standalone-citadel -n istio-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}'`

for CLUSTER in ${CLUSTER1_NAME} ${CLUSTER2_NAME} ${CLUSTER3_NAME}
do
  # Installing new citadel
  kubectl --context ${CLUSTER} delete  deployment  -n istio-system  istio-citadel
  sed -e "s/__CLUSTERNAME__/${CLUSTER}/g;s/__ROOTCA_HOST__/${rootca_host}/g" istio-citadel-new.yaml | kubectl --context ${CLUSTER} apply -f -
  kubectl --context ${CLUSTER} apply -f  istio-auto-injection.yaml || true
done