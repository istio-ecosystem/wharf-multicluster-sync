#!/bin/bash

if [ "$#" -eq 0 ]; 
then
    echo "Syntax: cleanup_cluster.sh <context>"
    exit 1    
fi

kubectl delete --context=$1 -f deploy.yaml
kubectl delete --context=$1 -n istio-system configmaps mc-configuration
kubectl --context=$1 -n istio-system patch service istio-ingressgateway --type=json --patch='[{"op": "test", "path": "/spec/ports/0/port", "value": 31444}, {"op": "remove", "path": "/spec/ports/0"}]' || true