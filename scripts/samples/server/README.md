# Installing
```
./create_ingress_direct_route.sh
./create_clientserver.yaml
```

# Testing
```
clientPod=`kubectl --context ${CLUSTER1_NAME} get po -l app=client -o jsonpath='{.items[0].metadata.name}'`
kubectl --context ${CLUSTER1_NAME} exec -it $clientPod -c client -- curl -s -o /dev/null -I -w "%{http_code}" http://server.default.svc.cluster.local/helloworld
```

# Deleting
```
./delete_clientserver.yaml
./delete_ingress_direct_route.sh
```
