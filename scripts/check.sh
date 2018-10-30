#!/bin/bash
set -o errexit

if [ -z "$1" ]; then
    echo "Usage: $0 <context>"
    exit 1
fi

CONTEXT=$1
ISTIO_NS=istio-system

SECRETS=$(kubectl --context "$CONTEXT" --namespace "$ISTIO_NS" get secrets --field-selector type=istio.io/key-and-cert -o=custom-columns=NAME:.metadata.name --no-headers=true)
# echo Secrets is $SECRETS
for SECRET in $SECRETS; do
	echo secret: $SECRET
	ROOT_MD5=$(kubectl --context "$CONTEXT" --namespace "$ISTIO_NS" get secret "$SECRET" -o yaml | grep root-cert.pem | awk '{ print $2 }' | base64 --decode | openssl md5)
	echo "   " MD5 of root-cert.pem is $ROOT_MD5
	CERT_CHAIN_LEN=$(kubectl --context "$CONTEXT" --namespace "$ISTIO_NS" get secret "$SECRET" -o yaml | grep cert-chain.pem | awk '{ print $2 }' | base64 --decode | grep "BEGIN CERTIFICATE" | wc -l) 
	echo "   " cert-chain.pem has $CERT_CHAIN_LEN entries
	CHAIN_MD5=$(kubectl --context "$CONTEXT" --namespace "$ISTIO_NS" get secret "$SECRET" -o yaml | grep cert-chain.pem | awk '{ print $2 }' | base64 --decode | openssl md5)
	echo "   " MD5 of cert-chain.pem is $CHAIN_MD5
	CHAIN_EXPIRY=$(kubectl --context "$CONTEXT" --namespace "$ISTIO_NS" get secret "$SECRET" -o yaml | grep cert-chain.pem | awk '{ print $2 }' | base64 --decode | openssl x509 -enddate -noout -checkend 0)
	if [ $? -ne 0 ]; then
		echo "   " cert-chain.pem EXPIRED ON $CHAIN_EXPIRY
	else
		echo "   " cert-chain.pem valid until $CHAIN_EXPIRY
	fi
	KEY_MD5=$(kubectl --context "$CONTEXT" --namespace "$ISTIO_NS" get secret "$SECRET" -o yaml | grep key.pem | awk '{ print $2 }' | base64 --decode | openssl md5)
	echo "   " MD5 of key.pem is $KEY_MD5
done

if kubectl --context "$CONTEXT" --namespace "$ISTIO_NS" get secret cacerts > /dev/null 2>/dev/null; then
  echo ""
  echo "special Opaque client secret: cacerts" 
  CACERT_CERT_CHAIN_EXPIRY=$(kubectl --context "$CONTEXT" --namespace "$ISTIO_NS" get secret cacerts -o yaml | grep cert-chain.pem | awk '{ print $2 }' | base64 --decode | openssl x509 -enddate -noout -checkend 0)
  if [ $? -ne 0 ]; then
	echo "   " cacerts/cert-chain.pem EXPIRED ON $CACERT_CERT_CHAIN_EXPIRY
  else
	echo "   " cacerts/cert-chain.pem valid until $CACERT_CERT_CHAIN_EXPIRY
  fi
else
  echo ""
  echo "cacerts does not exist (which is OK if this is the root CA cluster)"
fi
