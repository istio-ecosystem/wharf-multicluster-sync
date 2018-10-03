
# Generating Istio configuration from Service Exposition Policy and Binding using a Command Line

TODO replace 'go run' with binary, show more examples

```
cd cmd/mc-tool
go run main.go --filename ~/go/src/github.com/istio-ecosystem/wharf-multicluster-sync/docs/tutorial/bookinfo/ratings-exposure.yaml --cluster cluster1=1.2.3.4:80,cluster2=5.6.7.8:81
```