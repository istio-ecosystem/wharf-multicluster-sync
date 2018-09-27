#!/bin/bash
set -e

source ~/Sandbox/kube_context.sh


kubectl --context ${CLUSTER3_NAME} create  -f ./bookinfo-norating-noreviews.yaml
kubectl --context ${CLUSTER3_NAME} create  -f ./bookinfo-gateway.yaml
kubectl --context ${CLUSTER3_NAME} create  -f ./productpage-dr.yaml

IP=`kubectl --context $CLUSTER3_NAME get svc -n istio-system   istio-ingressgateway  -o jsonpath='{.status.loadBalancer.ingress[0].ip}'`


export INGRESS_HOST=$(kubectl --context $CLUSTER3_NAME  get po -l istio=ingressgateway -n istio-system -o 'jsonpath={.items[0].status.hostIP}')
#export INGRESS_HOST=$(kubectl --context $CLUSTER3_NAME  get svc istio-ingressgateway -n istio-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
export INGRESS_PORT=$(kubectl --context  $CLUSTER3_NAME -n istio-system get service istio-ingressgateway -o jsonpath='{.spec.ports[?(@.name=="http2")].nodePort}')
export GATEWAY_URL=$INGRESS_HOST:$INGRESS_PORT
echo " *** Accees Bookinfo at: http://"${GATEWAY_URL}"/productpage ***"

