#!/bin/bash
set +e

source ~/Sandbox/kube_context.sh

kubectl --context ${CLUSTER2_NAME} delete  -f ./bookinfo-reviews-v2-v3.yaml
kubectl --context ${CLUSTER3_NAME} delete -f ./bookinfo-ratings.yaml

kubectl --context ${CLUSTER3_NAME} delete -f ratings-exposure.yaml

