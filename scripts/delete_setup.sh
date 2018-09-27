#!/bin/bash
set +e

source ~/Sandbox/kube_context.sh

kubectl --context ${ROOTCA_NAME} delete  -f istio-citadel-standalone.yaml


for CLUSTER in ${CLUSTER1_NAME} ${CLUSTER2_NAME} ${CLUSTER3_NAME} ${ROOTCA_NAME} 
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

pkill main
rm ${DEMODIR}/clusters/*.log

echo "*** Make sure noone listening on the agent ports: "
lsof -nP -i4TCP:8997 | grep LISTEN
lsof -nP -i4TCP:8998 | grep LISTEN
lsof -nP -i4TCP:8999 | grep LISTEN
echo "*** Make sure noone listening on the agent ports, end. "
