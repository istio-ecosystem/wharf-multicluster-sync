#!/bin/bash
set -e

SCRIPTDIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )

source ~/Sandbox/kube_context.sh

kubectl --context ${CLUSTER1_NAME} get nodes
kubectl --context ${CLUSTER2_NAME} get nodes


kubectl --context ${CLUSTER1_NAME} apply -f ${SCRIPTDIR}/client.yaml
kubectl --context ${CLUSTER2_NAME} apply -f ${SCRIPTDIR}/client.yaml


#MBMBMB changes hostname below to ip

set -x
clientPod_cluster1=`kubectl --context ${CLUSTER1_NAME} get po -l app=client -o jsonpath='{.items[0].metadata.name}'`
clientPod_cluster2=`kubectl --context ${CLUSTER2_NAME} get po -l app=client -o jsonpath='{.items[0].metadata.name}'`
sleep 3

kubectl --context ${CLUSTER1_NAME} exec -it $clientPod_cluster1 -c curlclient -- curl -s -o /dev/null -I -w "%{http_code}" ratings:9080/ratings/0

kubectl --context ${CLUSTER2_NAME} exec -it $clientPod_cluster2 -c curlclient -- curl -s -o /dev/null -I -w "%{http_code}" ratings:9080/ratings/0
