#!/bin/bash
set -o errexit

if [ -z "$2" ]
  then
    echo "Syntax: deploy_cluster.sh client-cluster peer-cluster [peer-cluster]*"
    exit 1
fi

if [ -z "$3" ]
  then
    echo ""
  else
    echo "Unimplemented: Create config map for multiple services.  Use one peer for now."
    exit 1
fi

CLIENT_CLUSTER=$1
CLIENT_IP=`kubectl --context ${CLIENT_CLUSTER} get service istio-ingressgateway -n istio-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}'`
shift

# TODO Create SERVER_IP as list so we can create ConfigMap with list of peers
for SERVER_CLUSTER in "$@"
do
	SERVER_IP=`kubectl --context ${SERVER_CLUSTER} get service istio-ingressgateway -n istio-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}'`
	echo $CLIENT_CLUSTER is a client of $SERVER_CLUSTER with Ingress Gateway at $SERVER_IP
done

# Create ConfigMap to configure agent
set +e
cat <<EOF | kubectl --context $CLIENT_CLUSTER apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: mc-configuration
  namespace: default
  labels:
    istio: multi-cluster-agent
data:
  config.yaml: |
      ID: $CLIENT_CLUSTER
      GatewayIP: $CLIENT_IP
      GatewayPort: 80
      AgentPort: 8999
      TrustedPeers: []
      WatchedPeers:
      - ID: $SERVER_CLUSTER
        GatewayIP: $SERVER_IP
        GatewayPort: 80
        AgentIP: 1.2.3.4
        AgentPort: 80 
        ConnectionMode: live
EOF
set -e
	
# Create Pod
kubectl --context $CLIENT_CLUSTER apply -f deploy.yaml
kubectl --context $CLIENT_CLUSTER apply -f mc_gateways.yaml
