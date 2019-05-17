#!/bin/bash
set -e -x


DELETE_ROOTCA=
NAMESPACE=istio-system
declare -a CLUSTERS
while (( $# > 0 )); do
  case "${1}" in
    --root-ca|--rootca)
       ROOTCA="${2}"
       shift; shift
       ;;
    --delete-root-ca|--delete-rootca)
       DELETE_ROOTCA=true
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


rootca_host=`kubectl --context ${ROOTCA} get service standalone-citadel -n ${NAMESPACE} -o jsonpath='{.status.loadBalancer.ingress[0].ip}'`
# handle case of no LB (ex ICP)
if [[ -z $rootca_host ]]; then
  rootca_host=`kubectl --context ${ROOTCA} get nodes | grep proxy | awk '{print $1}'`
  rootca_port=`kubectl --context ${ROOTCA} get service standalone-citadel -n ${NAMESPACE} -o jsonpath='{.spec.ports[0].nodePort}'`
fi

for CLUSTER in "${CLUSTERS[@]}"; do
  SERVICE_ACCOUNT="istio-citadel-service-account-${CLUSTER}"
  CERT_NAME="istio.${SERVICE_ACCOUNT}"

  if [[ -n $rootca_port ]]; then
    sed -e "s/__ROOTCA_HOST__/${rootca_host}/g;s/__ROOTCA_PORT__/${rootca_port}/g" istio-citadel-new-svc-nolb.yaml \
      | kubectl --context ${CLUSTER} delete -f - --ignore-not-found
  else
    sed -e "s/__ROOTCA_HOST__/${rootca_host}/g" istio-citadel-new-svc-lb.yaml \
      | kubectl --context ${CLUSTER} delete -f --ignore-not-found
  fi
  sed -e "s/__CLUSTERNAME__/${CLUSTER}/g" istio-citadel.yaml \
    | kubectl --context ${CLUSTER} delete -f - --ignore-not-found

  kubectl --context ${CLUSTER} -n ${NAMESPACE} delete secret cacerts --ignore-not-found

  kubectl --context ${ROOTCA} -n ${NAMESPACE} delete serviceaccount ${SERVICE_ACCOUNT} --ignore-not-found
  kubectl --context ${ROOTCA} -n ${NAMESPACE} delete secret ${CERT_NAME} --ignore-not-found

  kubectl --context ${CLUSTER} apply -f istio-citadel-ondelete.yaml
done

if [[ -n "${DELETE_ROOTCA}" ]]; then
  kubectl --context ${ROOTCA} delete -f istio-citadel-standalone.yaml --ignore-not-found
fi


