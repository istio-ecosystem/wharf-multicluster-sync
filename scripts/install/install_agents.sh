#!/bin/bash
set -e

source ~/Sandbox/kube_context.sh

AGENDIR="./agents"
CLUSTERDIR="./clusters"

cd $AGENTDIR
export MC_STYLE=DIRECT_INGRESS


#echo "************ DID NOT start agents. ****************"
for CLUSTER in ${CLUSTER1_NAME} ${CLUSTER2_NAME} ${CLUSTER3_NAME}
do
  go run main.go -configJson $CLUSTERDIR/cluster_a.json --context ${CLUSTER} > $CLUSTERDIR/cluster_a.log 2>&1 &
done


