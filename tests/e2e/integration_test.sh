#!/bin/bash
set -e

if [ "$#" -ne 2 ]; then
    echo "usage: integration_test.sh cluster1 cluster2"
    exit 1
fi

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null && pwd )"
CLUSTER1=$1
CLUSTER2=$2

ROOTDIR="${DIR}/../.."
INSTALLDIR="${ROOTDIR}/docs/install"
TESTDIR="${ROOTDIR}/docs/tutorial/bookinfo"

cd ${INSTALLDIR}
./deploy_cluster.sh cluster2=$CLUSTER2
./deploy_cluster.sh cluster1=$CLUSTER1 cluster2=$CLUSTER2

kubectl --context $CLUSTER1 apply -f ${TESTDIR}/bookinfo-norating-noreviews.yaml
kubectl --context $CLUSTER1 apply -f ${TESTDIR}/bookinfo-gateway.yaml

export INGRESS_HOST=$(kubectl --context $CLUSTER1  get svc istio-ingressgateway -n istio-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
export INGRESS_PORT=$(kubectl --context $CLUSTER1 -n istio-system get service istio-ingressgateway -o jsonpath='{.spec.ports[?(@.name=="http2")].nodePort}')
export GATEWAY_URL=$INGRESS_HOST:$INGRESS_PORT
echo "Accees Bookinfo at: http://"${GATEWAY_URL}"/productpage"

until curl -s -o /dev/null -I -w "%{http_code}" http://$GATEWAY_URL/productpage | grep '200' &> /dev/null
do
    echo "waiting for the productpage to be served ..."
    sleep 1
done
echo "*** The Product page is up. ***"

kubectl --context $CLUSTER2 apply -f ${TESTDIR}/bookinfo-reviews-v1.yaml
kubectl --context $CLUSTER2 apply -f ${TESTDIR}/reviews-exposure.yaml

#while curl http://$GATEWAY_URL/productpage | grep 'Sorry' &> /dev/null
for LOOP in 1 2 3 4 5
do
    if curl http://$GATEWAY_URL/productpage | grep 'Sorry' &> /dev/null
    then
       sleep 1
    else
       SUCCESS="1"
       break
    fi
done


if [ "$SUCCESS" = "1" ]
then
       echo "*** Reviews from the second cluster is accessed. Inter-cluster test succeeded. ***"
       exit 0
else
       echo "=== :( :( :( :( :( Failed :( :( :( :( :( ==="
       exit 1
fi


if [ -n "$CLEANUP" ]
then
  echo "Cleanup..."
  kubectl --context $CLUSTER2 delete -f ${TESTDIR}/reviews-exposure.yaml
  kubectl --context $CLUSTER1 delete service reviews
  kubectl --context $CLUSTER1 delete -f ${TESTDIR}/bookinfo-norating-noreviews.yaml
  kubectl --context $CLUSTER1 delete -f ${TESTDIR}/bookinfo-gateway.yaml
  kubectl --context $CLUSTER2 delete -f ${TESTDIR}/bookinfo-reviews-v1.yaml

  kubectl --context $CLUSTER1 delete -f deploy.yaml 
  kubectl --context $CLUSTER2 delete -f deploy.yaml 
  kubectl --context $CLUSTER1 delete cm mc-configuration 
  kubectl --context $CLUSTER2 delete cm mc-configuration 
fi