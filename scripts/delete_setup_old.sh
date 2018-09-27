#!/bin/bash
set +e

source ~/Sandbox/kube_context.sh

kubectl --context ${ROOTCA_NAME} delete  -f istio-citadel-standalone.yaml
kubectl --context ${ROOTCA_NAME} delete serviceaccount -n istio-system istio-citadel-service-account-${CLUSTER1_ID}
kubectl --context ${ROOTCA_NAME} delete serviceaccount -n istio-system istio-citadel-service-account-${CLUSTER2_ID}

for CLUSTER in ${CLUSTER1_NAME} ${CLUSTER2_NAME} ${ROOTCA_NAME} ${CLUSTER3_NAME}
do
  kubectl --context ${CLUSTER} delete namespace istio-system
  kubectl --context ${CLUSTER} delete  -f ${ISTIODIR}/install/kubernetes/istio-demo-auth.yaml
  kubectl --context ${CLUSTER} delete  -f istio-citadel-new.yaml
done

rm ${DEMODIR}/clustes/*.log