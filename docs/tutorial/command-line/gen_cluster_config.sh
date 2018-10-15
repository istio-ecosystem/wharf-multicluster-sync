#!/bin/bash

set -o errexit
#set -o xtrace

INTERMESH_PORT=31444

if ! [ -z "$3" ]; then
    echo "Unimplemented: Create config map for multiple services. Use one peer for now."
    exit 1
fi

CLIENT_CLUSTER=$1
CLIENT_ID=$(echo $CLIENT_CLUSTER | cut -f1 -d=)
CLIENT_NAME=$(echo $CLIENT_CLUSTER | cut -f2 -d=)
if [ -z "$CLIENT_NAME" ]; then
	CLIENT_NAME=$CLIENT_CLUSTER
else
	echo $CLIENT_NAME will have role $CLIENT_ID
fi

CLIENT_IP=`kubectl --context ${CLIENT_NAME} get service istio-ingressgateway -n istio-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}'`
shift

if [ "$#" -eq 0 ]; 
then
  PEERS="WatchedPeers: []"
else
# TODO Create SERVER_IP as list so we can create ConfigMap with list of peers
for SERVER_CLUSTER in "$@"
do
	SERVER_ID=$(echo $SERVER_CLUSTER | cut -f1 -d=)
	SERVER_NAME=$(echo $SERVER_CLUSTER | cut -f2 -d=)
	if [ -z "$SERVER_NAME" ]; then
		SERVER_NAME=$SERVER_CLUSTER
	else
		echo $SERVER_NAME will have role $SERVER_ID
	fi

	SERVER_IP=`kubectl --context ${SERVER_NAME} get service istio-ingressgateway -n istio-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}'`
	# SERVER_AGENT_IP=`kubectl --context ${SERVER_NAME} get service mc-agent -n "$AGENT_NS" -o jsonpath='{.status.loadBalancer.ingress[0].ip}'`
	SERVER_AGENT_IP=$SERVER_IP
	echo $CLIENT_NAME \($CLIENT_ID\) is a client of $SERVER_NAME \($SERVER_ID\) with Ingress Gateway at $SERVER_IP
done

  PEERS=$(cat <<-END
WatchedPeers:
      - ID: $SERVER_ID
        GatewayIP: $SERVER_IP
        GatewayPort: $INTERMESH_PORT
        AgentIP: $SERVER_AGENT_IP
        AgentPort: 80
        ConnectionMode: $CONNECTION_MODE
END
)
fi

OUTPUT=${CLIENT_ID}-conf.yaml

# Create ConfigMap to configure agent
set +e
cat <<EOF > $OUTPUT
apiVersion: v1
kind: ConfigMap
metadata:
  name: ${CLIENT_ID}
data:
  config.yaml: |
      ID: $CLIENT_ID
      GatewayIP: $CLIENT_IP
      GatewayPort: $INTERMESH_PORT
      AgentPort: 8999
      TrustedPeers:
      - "*"
      $PEERS
EOF
set -e

echo Generated $OUTPUT
