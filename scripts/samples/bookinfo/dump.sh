#!/bin/bash
set -e

source ~/Sandbox/kube_context.sh

kubectl --context ${CLUSTER1_NAME} get serviceentry  service-entry-clusterb.myorg-services -o json > dump.json
kubectl --context ${CLUSTER1_NAME} get destinationrule dest-rule-clusterb.myorg-services-default -o json >> dump.json

kubectl --context ${CLUSTER2_NAME} get destinationrule dest-rule-reviews-default-notls -o json >> dump.json
kubectl --context ${CLUSTER2_NAME} get gateway istio-ingressgateway-reviews-default -o json >> dump.json
kubectl --context ${CLUSTER2_NAME} get virtualservice ingressgateway-to-reviews-default  -o json >> dump.json
