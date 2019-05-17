#!/bin/bash
set -e -x

function usage() {
  echo "Usage $0 --root-ca root_ca_context [--install-root-ca] k8s_context*"
  exit 1
}

INSTALL_ROOTCA=
NAMESPACE=istio-system
declare -a CLUSTERS
while (( $# > 0 )); do
  case "${1}" in
    --root-ca|--rootca)
       ROOTCA="${2}"
       shift; shift
       ;;
    --install-root-ca|--install-rootca)
       INSTALL_ROOTCA=true
       shift
       ;;
    --install-namespace)
       NAMESPACE="${2}"
       shift; shift
       ;;
    *) CLUSTERS+=( "${1}" )
       shift
       ;;
  esac
done

B64_DECODE=${BASE64_DECODE:-base64 --decode}

function install_root_ca() {
  echo "Installing root CA on cluster ${ROOTCA}"
  kubectl --context ${ROOTCA} apply -f istio-citadel-standalone.yaml
}

function configure_client_cluster() {
  local CLUSTER="${1}"
  echo "Configuring istio in cluster '${CLUSTER}' to use root CA on '${ROOTCA}'"

  # create service account (delete it and its associated secret first, if they exist)
  SERVICE_ACCOUNT=$(echo "istio-citadel-service-account-$CLUSTER" | tr '[:upper:]' '[:lower:]') #lower case to make it valid service account name
  CERT_NAME="istio.${SERVICE_ACCOUNT}"

  kubectl --context ${ROOTCA} -n ${NAMESPACE} delete serviceaccount ${SERVICE_ACCOUNT} --wait=true --ignore-not-found
  kubectl --context ${ROOTCA} -n ${NAMESPACE} delete secret ${CERT_NAME} --wait=true --ignore-not-found

  kubectl --context ${ROOTCA} -n ${NAMESPACE} create serviceaccount ${SERVICE_ACCOUNT}
  until kubectl --context ${ROOTCA} get -n ${NAMESPACE} secret ${CERT_NAME}
  do
    echo "waiting for the cert to be generated ..."
    sleep 1
  done

  # create secret/cacerts (delete it first, if it exists)
  kubectl --context ${CLUSTER} delete secret cacerts -n ${NAMESPACE} --ignore-not-found

  DIR="/tmp/ca/${CLUSTER}"
  mkdir -p ${DIR}; rm -rf ${DIR}/*
  kubectl --context ${ROOTCA} get -n ${NAMESPACE} secret $CERT_NAME -o jsonpath='{.data.root-cert\.pem}' | $B64_DECODE   > ${DIR}/root-cert.pem
  kubectl --context ${ROOTCA} get -n ${NAMESPACE} secret $CERT_NAME -o jsonpath='{.data.cert-chain\.pem}' | $B64_DECODE  > ${DIR}/cert-chain.pem
  kubectl --context ${ROOTCA} get -n ${NAMESPACE} secret $CERT_NAME -o jsonpath='{.data.key\.pem}' | $B64_DECODE   > ${DIR}/ca-key.pem
  cp ${DIR}/cert-chain.pem ${DIR}/ca-cert.pem

  kubectl --context ${CLUSTER} create secret generic cacerts -n ${NAMESPACE} \
          --from-file=${DIR}/ca-cert.pem --from-file=${DIR}/ca-key.pem \
          --from-file=${DIR}/root-cert.pem --from-file=${DIR}/cert-chain.pem

  # recreate istio-citadel deployment to use upstream standalone citadel
  kubectl --context ${CLUSTER} delete  deployment  -n ${NAMESPACE}  istio-citadel --ignore-not-found
  standalone_svc="istio-citadel-new-svc-lb.yaml"
  if [[ -n $rootca_port ]]; then
    standalone_svc=istio-citadel-new-svc-nolb.yaml
  fi
  sed -e "s/__ROOTCA_HOST__/${rootca_host}/g;s/__ROOTCA_PORT__/${rootca_port}/g" ${standalone_svc} | tee ${DIR}/svc.yaml | kubectl --context ${CLUSTER} apply -f -
  sed -e "s/__SERVICE_ACCOUNT__/${SERVICE_ACCOUNT}/g" istio-citadel.yaml | tee ${DIR}/istio-citadel.yaml | kubectl --context ${CLUSTER} apply -f -

  # Do basic validatation of config -- check that istio-citadel starts
  end=$((SECONDS+10))
  until (( $SECONDS > $end )) || \
        [[ 1 == $(kubectl --context ${CLUSTER} -n ${NAMESPACE} get deploy istio-citadel -o jsonpath='{.status.readyReplicas}') ]]
  do
    sleep 3
  done
  if [[ "1" != $(kubectl --context ${CLUSTER} -n ${NAMESPACE} get deploy istio-citadel -o jsonpath='{.status.readyReplicas}') ]]; then
    echo "ERROR: istio-citadel not started correctly. Please check configuration."
    exit 1
  fi
}

############################################
######## main
############################################
if [[ -z "${ROOTCA}" ]]; then
  usage
fi
echo "Using root CA on cluster '$ROOTCA'"

if [[ -n ${INSTALL_ROOTCA} ]]; then
  install_root_ca 
fi
 
# Identify root CA ip (and port, if needed)
rootca_host=`kubectl --context ${ROOTCA} get service standalone-citadel -n ${NAMESPACE} -o jsonpath='{.status.loadBalancer.ingress[0].ip}'`
# handle case of no LB (ex ICP)
if [[ -z $rootca_host ]]; then
  rootca_host=`kubectl --context ${ROOTCA} get nodes | grep proxy | awk '{print $1}'`
  rootca_port=`kubectl --context ${ROOTCA} get service standalone-citadel -n ${NAMESPACE} -o jsonpath='{.spec.ports[0].nodePort}'`
fi

for CLUSTER in  "${CLUSTERS[@]}"; do
  configure_client_cluster ${CLUSTER}
done
