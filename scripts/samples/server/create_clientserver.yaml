#!/bin/bash
set -e

SCRIPTDIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )

source ~/Sandbox/kube_context.sh

kubectl --context ${CLUSTER1_NAME} get nodes
kubectl --context ${CLUSTER2_NAME} get nodes


kubectl --context ${CLUSTER1_NAME} apply -f ${SCRIPTDIR}/client.yaml
kubectl --context ${CLUSTER2_NAME} apply -f ${SCRIPTDIR}/server.yaml

kubectl --context ${CLUSTER1_NAME} apply -f ${SCRIPTDIR}/client-service.yaml

#MBMBMB changes hostname below to ip

set -x
clientPod=`kubectl --context ${CLUSTER1_NAME} get po -l app=client -o jsonpath='{.items[0].metadata.name}'`
sleep 3

output=`kubectl --context ${CLUSTER1_NAME} exec -it $clientPod -c client -- curl -s -o /dev/null -I -w "%{http_code}" http://server.default.svc.cluster.local/helloworld`
success=0
if [ "$output" != "200" ]; then
    echo "Failed to reach remote server server.ns2.svc.cluster.global"
    success=1
else
    echo "Successfully connnected to remote server server.ns2.svc.cluster.global"
    success=0
fi


echo "That is it for now."