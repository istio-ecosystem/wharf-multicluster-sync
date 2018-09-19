#!/bin/bash
set -e

source ~/Sandbox/kube_context.sh


echo "Make sure the cluster jsons have correct ip addresses: "
CLUSTERA="127.0.0.1"
echo "cluster_a: " "${CLUSTERA}"
echo
CLUSTERB=`kubectl --context ${CLUSTER2_NAME} get service istio-ingressgateway -n istio-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}'`
echo "cluster_b:"  "${CLUSTERB}"
echo
CLUSTERC=`kubectl --context ${CLUSTER3_NAME} get service istio-ingressgateway -n istio-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}'`
echo "cluster_c:" "${CLUSTERC}"
echo


sed -e "s/__CLUSTERA_IP__/${CLUSTERA}/g;s/__CLUSTERB_IP__/${CLUSTERB}/g;s/__CLUSTERC_IP__/${CLUSTERC}/g" clusters/cluster_a.tmp > clusters/cluster_a.j\
son
sed -e "s/__CLUSTERA_IP__/${CLUSTERA}/g;s/__CLUSTERB_IP__/${CLUSTERB}/g;s/__CLUSTERC_IP__/${CLUSTERC}/g" clusters/cluster_b.tmp > clusters/cluster_b.j\
son
sed -e "s/__CLUSTERA_IP__/${CLUSTERA}/g;s/__CLUSTERB_IP__/${CLUSTERB}/g;s/__CLUSTERC_IP__/${CLUSTERC}/g" clusters/cluster_c.tmp > clusters/cluster_c.j\
son

echo "That is it for now."


cd $AGENTDIR
export MC_STYLE=DIRECT_INGRESS
go run main.go -configJson $DEMODIR/clusters/cluster_a.json --context $CLUSTER1_NAME > $DEMODIR/clusters/cluster_a.log 2>&1 &
go run main.go -configJson $DEMODIR/clusters/cluster_b.json --context $CLUSTER2_NAME > $DEMODIR/clusters/cluster_b.log 2>&1 &
go run main.go -configJson $DEMODIR/clusters/cluster_c.json --context $CLUSTER3_NAME > $DEMODIR/clusters/cluster_c.log 2>&1 &

