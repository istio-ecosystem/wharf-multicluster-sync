#!/bin/bash
set -e

source ~/Sandbox/kube_context.sh

kubectl --context ${CLUSTER2_NAME} apply  -f ./bookinfo-reviews-v2-v3.yaml

kubectl --context ${CLUSTER3_NAME} apply  -f ./bookinfo-ratings.yaml
kubectl --context ${CLUSTER3_NAME} apply -f ratings-exposure.yaml

