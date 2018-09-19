#!/bin/bash
set -e

source ~/Sandbox/kube_context.sh

kubectl --context ${CLUSTER1_NAME} get serviceentry  service-entry-reviews -o json > config_dump.json || true
kubectl --context ${CLUSTER1_NAME} get destinationrule dest-rule-reviews -o json >> config_dump.json || true

kubectl --context ${CLUSTER2_NAME} get destinationrule dest-rule-reviews-default-notls -o json >> config_dump.json || true
kubectl --context ${CLUSTER2_NAME} get gateway istio-ingressgateway-reviews-default -o json >> config_dump.json || true
kubectl --context ${CLUSTER2_NAME} get virtualservice ingressgateway-to-reviews-default  -o json >> config_dump.json || true
