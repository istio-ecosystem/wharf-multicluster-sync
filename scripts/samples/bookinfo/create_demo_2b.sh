#!/bin/bash
set -e

source ~/Sandbox/kube_context.sh

kubectl --context ${CLUSTER3_NAME} apply -f ratings-exposure.yaml

