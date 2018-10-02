#!/bin/bash

if [ "$#" -eq 0 ]; 
then
    echo "Syntax: cleanup_cluster.sh <context>"
    exit 1    
fi

kubectl delete --context=$1 -f deploy.yaml
kubectl delete --context=$1 configmaps mc-configuration