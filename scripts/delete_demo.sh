#!/bin/bash
set +e

source ~/Sandbox/kube_context.sh

kubectl --context ${ROOTCA_NAME} delete  -f istio-citadel-standalone.yaml


for CLUSTER in ${CLUSTER1_NAME} ${CLUSTER2_NAME} ${ROOTCA_NAME} ${CLUSTER3_NAME}
do
  echo "Deleting Istio resources on" $CLUSTER 
  kubectl --context ${ROOTCA_NAME} delete serviceaccount -n istio-system istio-citadel-service-account-${CLUSTER}
  kubectl --context ${CLUSTER} delete  -f ${ISTIODIR}/install/kubernetes/istio-demo-auth.yaml
  kubectl --context ${CLUSTER} delete  -f istio-citadel-new.yaml
  kubectl --context ${CLUSTER} delete namespace istio-system
  kubectl --context ${CLUSTER} delete RemoteServiceBinding,ServiceExpositionPolicy --all
done

for CLUSTER in ${CLUSTER1_NAME} ${CLUSTER2_NAME} ${ROOTCA_NAME} ${CLUSTER3_NAME}
do
  echo "Deleting IstioMC resources on" $CLUSTER 
  kubectl --context ${CLUSTER} delete ServiceEntry,Gateway,DestinationRule,VirtualService --all
done

pkill go
rm ${DEMODIR}/clustes/*.log