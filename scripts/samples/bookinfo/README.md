# Install
makesure agent running

```
./create_reviews.sh
```


# test

export INGRESS_HOST=$(kubectl --context $CLUSTER1_NAME  get po -l istio=ingressgateway -n istio-system -o 'jsonpath={.items[0].status.hostIP}')
export INGRESS_PORT=$(kubectl --context  $CLUSTER1_NAME -n istio-system get service istio-ingressgateway -o jsonpath='{.spec.ports[?(@.name=="http2")].nodePort}')
export GATEWAY_URL=$INGRESS_HOST:$INGRESS_PORT
echo $GATEWAY_URL


http://184.172.242.125:31380/productpage


kubectl --context $CLUSTER1_NAME exec -it client-d9f48b488-5jvpb  -c client -- curl -vvv  reviews:9080/reviews/0


# Debug

run dump.sh and inspect the output in dump.json. It contains the info on all rekated resources.