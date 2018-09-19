#!/bin/bash
set +e

source ~/Sandbox/kube_context.sh

kubectl --context ${CLUSTER2_NAME} delete -f reviews-exposure-starter-v1-v2.yaml
kubectl --context ${CLUSTER2_NAME} delete -f reviews-exposure-v1-v2.yaml

