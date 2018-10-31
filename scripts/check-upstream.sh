#!/bin/bash
set -o errexit

if [ -z "$1" ]; then
    echo "Usage: $0 <context>"
    exit 1
fi

CONTEXT=$1
ISTIO_NS=istio-system

# Find real address of upstream CA
# kubectl --context "$CONTEXT" -n "$ISTIO_NS" get endpoints istio-standalone-citadel -o yaml
CA_HOST=$(kubectl --context "$CONTEXT" -n "$ISTIO_NS" get endpoints istio-standalone-citadel -o jsonpath='{.subsets[0].addresses[0].ip}')
CA_PORT=$(kubectl --context "$CONTEXT" -n "$ISTIO_NS" get endpoints istio-standalone-citadel -o jsonpath='{.subsets[0].ports[0].port}')

# Get secrets needed for upstream CA
for PEMFILE in root-cert.pem ca-key.pem cert-chain.pem; do
   # echo '$PEMFILE' is $PEMFILE
   kubectl --context $CONTEXT -n $ISTIO_NS get secret cacerts -o yaml | grep $PEMFILE | awk '{ print $2 }' | base64 --decode > /tmp/$PEMFILE  
done
 
echo openssl s_client -connect "$CA_HOST:$CA_PORT" -CAfile /tmp/root-cert.pem -key /tmp/ca-key.pem -cert /tmp/cert-chain.pem
openssl s_client -connect "$CA_HOST:$CA_PORT" -CAfile /tmp/root-cert.pem -key /tmp/ca-key.pem -cert /tmp/cert-chain.pem
echo result is $?
