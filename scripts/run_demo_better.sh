#!/bin/bash
set -e

source ~/Sandbox/kube_context.sh

SCRIPTDIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )
ISTIODIR="/Users/mb/Repos/istio-1.0.2"

# To make sure we can ccess all clusters
kubectl --context ${CLUSTER1_NAME} get nodes
kubectl --context ${CLUSTER2_NAME} get nodes
kubectl --context ${CLUSTER3_NAME} get nodes
kubectl --context ${ROOTCA_NAME} get nodes

# Install the ROOT CA
kubectl --context ${ROOTCA_NAME} apply -f istio-citadel-standalone.yaml
kubectl --context ${ROOTCA_NAME} -n istio-system create serviceaccount istio-citadel-service-account-${CLUSTER1_NAME} 
kubectl --context ${ROOTCA_NAME} -n istio-system create serviceaccount istio-citadel-service-account-${CLUSTER2_NAME}
kubectl --context ${ROOTCA_NAME} -n istio-system create serviceaccount istio-citadel-service-account-${CLUSTER3_NAME}
rootca_host=`kubectl --context ${ROOTCA_NAME} get service standalone-citadel -n istio-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}'`


# Installing Cluster 
for CLUSTER in ${CLUSTER1_NAME} ${CLUSTER2_NAME} ${CLUSTER3_NAME}
do
  echo "on cluster" $CLUSTER " ..................................................................."
  # Installing Istio
  kubectl --context ${CLUSTER} create namespace istio-system || true
  kubectl --context ${CLUSTER} apply -f ${ISTIODIR}/install/kubernetes/helm/istio/templates/crds.yaml
  ${SCRIPTDIR}/provision_cluster_int_ca.sh $ROOTCA_NAME $CLUSTER $CLUSTER
  kubectl --context ${CLUSTER} apply -f ${ISTIODIR}/install/kubernetes/istio-demo-auth.yaml
  sleep 5
  # Installing new citadel
  kubectl --context ${CLUSTER} delete  deployment  -n istio-system   istio-citadel
  sed -e "s/__CLUSTERNAME__/${CLUSTER}/g;s/__ROOTCA_HOST__/${rootca_host}/g" istio-citadel-new.yaml | kubectl --context ${CLUSTER} apply -f -
  kubectl --context ${CLUSTER} apply -f  istio-auto-injection.yaml || true
done


echo "Make sure the cluster jsons have correct ip addresses: "
CLUSTERA="127.0.0.1"
echo "cluster_a:" "${CLUSTERA}"
CLUSTERB=`kubectl --context ${CLUSTER2_NAME} get service istio-ingressgateway -n istio-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}'`
echo "cluster_b:"  "${CLUSTERB}"
CLUSTERC=`kubectl --context ${CLUSTER3_NAME} get service istio-ingressgateway -n istio-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}'`
echo "cluster_c:" "${CLUSTERC}"

sed -e "s/__CLUSTERA_IP__/${CLUSTERA}/g;s/__CLUSTERB_IP__/${CLUSTERB}/g;s/__CLUSTERC_IP__/${CLUSTERC}/g" clusters/cluster_a.tmp > clusters/cluster_a.json
sed -e "s/__CLUSTERA_IP__/${CLUSTERA}/g;s/__CLUSTERB_IP__/${CLUSTERB}/g;s/__CLUSTERC_IP__/${CLUSTERC}/g" clusters/cluster_b.tmp > clusters/cluster_b.json
sed -e "s/__CLUSTERA_IP__/${CLUSTERA}/g;s/__CLUSTERB_IP__/${CLUSTERB}/g;s/__CLUSTERC_IP__/${CLUSTERC}/g" clusters/cluster_c.tmp > clusters/cluster_c.json

echo "That is it for now."

cd $AGENTDIR
export MC_STYLE=DIRECT_INGRESS
go run main.go -configJson $DEMODIR/clusters/cluster_a.json --context $CLUSTER1_NAME > $DEMODIR/clusters/cluster_a.log 2>&1 &
go run main.go -configJson $DEMODIR/clusters/cluster_b.json --context $CLUSTER2_NAME > $DEMODIR/clusters/cluster_b.log 2>&1 &
go run main.go -configJson $DEMODIR/clusters/cluster_c.json --context $CLUSTER3_NAME > $DEMODIR/clusters/cluster_c.log 2>&1 &

