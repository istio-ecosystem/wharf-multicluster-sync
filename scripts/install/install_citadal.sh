#!/bin/bash
set -e

source ./kube_context.sh

# To make sure we can ccess all clusters
kubectl --context ${CLUSTER0} get nodes
kubectl --context ${CLUSTER1} get nodes
kubectl --context ${CLUSTER2} get nodes
kubectl --context ${CLUSTER3} get nodes

# Install the ROOT CA
kubectl --context ${ROOTCA} apply -f istio-citadel-standalone.yaml
rootca_host=`kubectl --context ${ROOTCA} get service standalone-citadel -n istio-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}'`

NAMESPACE="istio-system"
B64_DECODE=${BASE64_DECODE:-base64 --decode}

for CLUSTER in ${CLUSTER1} ${CLUSTER2}
do
  kubectl --context ${ROOTCA} -n istio-system create serviceaccount istio-citadel-service-account-${CLUSTER} || true

  SERVICE_ACCOUNT="istio-citadel-service-account-${CLUSTER}"
  CERT_NAME="istio.${SERVICE_ACCOUNT}"
  DIR="/tmp/ca/${CLUSTER}"
  mkdir -p $DIR

  until kubectl --context ${ROOTCA} get -n ${NAMESPACE} secret ${CERT_NAME}
  do
    echo "waiting for the cert to be generated ..."
    sleep 1
  done

  kubectl --context ${ROOTCA} get -n ${NAMESPACE} secret $CERT_NAME -o jsonpath='{.data.root-cert\.pem}' | $B64_DECODE   > ${DIR}/root-cert.pem
  kubectl --context ${ROOTCA} get -n ${NAMESPACE} secret $CERT_NAME -o jsonpath='{.data.cert-chain\.pem}' | $B64_DECODE  > ${DIR}/cert-chain.pem
  kubectl --context ${ROOTCA} get -n ${NAMESPACE} secret $CERT_NAME -o jsonpath='{.data.key\.pem}' | $B64_DECODE   > ${DIR}/ca-key.pem
  cp ${DIR}/cert-chain.pem ${DIR}/ca-cert.pem

  kubectl --context ${CLUSTER} create secret generic cacerts -n istio-system \
          --from-file=${DIR}/ca-cert.pem --from-file=${DIR}/ca-key.pem \
          --from-file=${DIR}/root-cert.pem --from-file=${DIR}/cert-chain.pem


  kubectl --context ${CLUSTER} delete  deployment  -n istio-system  istio-citadel
  sed -e "s/__CLUSTERNAME__/${CLUSTER}/g;s/__ROOTCA_HOST__/${rootca_host}/g" istio-citadel-new.yaml | kubectl --context ${CLUSTER} apply -f -
  kubectl --context ${CLUSTER} apply -f  istio-auto-injection.yaml || true
done