#!/bin/bash
set -e

source ~/Sandbox/kube_context.sh

# To make sure we can ccess all clusters
kubectl --context ${CLUSTER1_NAME} get nodes
kubectl --context ${CLUSTER2_NAME} get nodes
kubectl --context ${CLUSTER3_NAME} get nodes
kubectl --context ${ROOTCA_NAME} get nodes

# Install the ROOT CA
kubectl --context ${ROOTCA_NAME} apply -f istio-citadel-standalone.yaml
rootca_host=`kubectl --context ${ROOTCA_NAME} get service standalone-citadel -n istio-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}'`

NAMESPACE="istio-system"
B64_DECODE=${BASE64_DECODE:-base64 --decode}

for CLUSTER in ${CLUSTER1_NAME} ${CLUSTER2_NAME} ${CLUSTER3_NAME}
do
  SERVICE_ACCOUNT="istio-citadel-service-account-${CLUSTER}"
  CERT_NAME="istio.${SERVICE_ACCOUNT}"
  DIR="/tmp/ca/${CLUSTER}"
  mkdir -p $DIR
  kubectl --context ${ROOTCA_NAME} get -n ${NAMESPACE} secret $CERT_NAME -o jsonpath='{.data.root-cert\.pem}' | $B64_DECODE   > ${DIR}/root-cert.pem
  kubectl --context ${ROOTCA_NAME} get -n ${NAMESPACE} secret $CERT_NAME -o jsonpath='{.data.cert-chain\.pem}' | $B64_DECODE  > ${DIR}/cert-chain.pem
  kubectl --context ${ROOTCA_NAME} get -n ${NAMESPACE} secret $CERT_NAME -o jsonpath='{.data.key\.pem}' | $B64_DECODE   > ${DIR}/ca-key.pem
  cp ${DIR}/cert-chain.pem ${DIR}/ca-cert.pem
  

  kubectl --context ${ROOTCA_NAME} -n istio-system create serviceaccount istio-citadel-service-account-${CLUSTER} 
  # Installing new citadel
  kubectl --context ${CLUSTER} delete  deployment  -n istio-system  istio-citadel
  sed -e "s/__CLUSTERNAME__/${CLUSTER}/g;s/__ROOTCA_HOST__/${rootca_host}/g" istio-citadel-new.yaml | kubectl --context ${CLUSTER} apply -f -
  kubectl --context ${CLUSTER} apply -f  istio-auto-injection.yaml || true
done