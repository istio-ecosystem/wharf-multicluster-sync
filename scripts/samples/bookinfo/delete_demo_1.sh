#!/bin/bash
set +e

source ~/Sandbox/kube_context.sh


kubectl --context ${CLUSTER1_NAME} delete  -f ./bookinfo-norating-noreviews.yaml || true
kubectl --context ${CLUSTER1_NAME} delete  -f ./bookinfo-gateway.yaml || true
kubectl --context ${CLUSTER1_NAME} delete  -f ./productpage-dr.yaml || true

kubectl --context ${CLUSTER2_NAME} delete  -f ./bookinfo-reviewsonly.yaml || true
#Make sure the gateway IP in cluster json is correct (kubectl --context $CLUSTER2_NAME  get svc --all-namespaces  | grep ingress), then:
kubectl --context ${CLUSTER2_NAME} delete  -f ./reviews-exposure.yaml || true
kubectl --context ${CLUSTER1_NAME} delete  -f ./reviews-selectorless-service.yaml || true


kubectl --context $CLUSTER1_NAME delete remoteservicebinding,ServiceExpositionPolicy  --all
kubectl --context $CLUSTER2_NAME delete remoteservicebinding,ServiceExpositionPolicy  --all


kubectl --context $CLUSTER1_NAME get remoteservicebinding 
kubectl --context $CLUSTER2_NAME get ServiceExpositionPolicy 

