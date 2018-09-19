#!/bin/bash
set -e

SCRIPTDIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )
CLUSTER1_ID="test-c1"
CLUSTER2_ID="test-c2"
ROOTCA_ID="root-ca"

#CLUSTER1_NAME="kubernetes-admin@kubernetes"
source ~/Sandbox/kube_context.sh


remote_ingress_gateway_lbhost=`kubectl --context ${CLUSTER2_NAME} get service istio-ingressgateway -n istio-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}'`

sed -e s/__REPLACEME__/${remote_ingress_gateway_lbhost}/g ${SCRIPTDIR}/ingress-direct-demo-binding.yaml | kubectl --context ${CLUSTER1_NAME} apply -f -
#kubectl --context ${CLUSTER1_NAME} apply -f ${SCRIPTDIR}/client.yaml

kubectl --context ${CLUSTER2_NAME} apply -f ${SCRIPTDIR}/ingress-direct-demo-exposure.yaml
#kubectl --context ${CLUSTER2_NAME} apply -f ${SCRIPTDIR}/server.yaml
