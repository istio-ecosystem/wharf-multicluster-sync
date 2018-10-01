#!/bin/bash
set -e

source ./kube_context.sh

# To make sure we can ccess all clusters
kubectl --context ${CLUSTER1_NAME} get nodes
kubectl --context ${CLUSTER2_NAME} get nodes
kubectl --context ${CLUSTER3_NAME} get nodes
kubectl --context ${ROOTCA_NAME} get nodes

# Install the ROOT CA
kubectl --context ${ROOTCA_NAME} apply -f istio-citadel-standalone.yaml

NAMESPACE="istio-system"
B64_DECODE=${BASE64_DECODE:-base64 --decode}

for CLUSTER in ${CLUSTER1_NAME} ${CLUSTER2_NAME} ${CLUSTER3_NAME}
do
  kubectl --context ${ROOTCA_NAME} -n istio-system create serviceaccount istio-citadel-service-account-${CLUSTER} || true
  ###
  ### kubectl --context ${CLUSTER} apply -f ${ISTIODIR}/install/kubernetes/helm/istio/templates/crds.yaml
  sleep 10

  SERVICE_ACCOUNT="istio-citadel-service-account-${CLUSTER}"
  CERT_NAME="istio.${SERVICE_ACCOUNT}"
  DIR="/tmp/ca/${CLUSTER}"
  mkdir -p $DIR

  kubectl --context ${ROOTCA_NAME} get -n ${NAMESPACE} secret $CERT_NAME -o jsonpath='{.data.root-cert\.pem}' | $B64_DECODE   > ${DIR}/root-cert.pem
  kubectl --context ${ROOTCA_NAME} get -n ${NAMESPACE} secret $CERT_NAME -o jsonpath='{.data.cert-chain\.pem}' | $B64_DECODE  > ${DIR}/cert-chain.pem
  kubectl --context ${ROOTCA_NAME} get -n ${NAMESPACE} secret $CERT_NAME -o jsonpath='{.data.key\.pem}' | $B64_DECODE   > ${DIR}/ca-key.pem
  cp ${DIR}/cert-chain.pem ${DIR}/ca-cert.pem

  kubectl --context ${CLUSTER} create secret generic cacerts -n istio-system \
          --from-file=${DIR}/ca-cert.pem --from-file=${DIR}/ca-key.pem \
          --from-file=${DIR}/root-cert.pem --from-file=${DIR}/cert-chain.pem
done