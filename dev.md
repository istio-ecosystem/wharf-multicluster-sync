# Developer instructions

To generate Go code for the .proto files, go to the _api_ directory and `make`.

To bring in new Go dependencies use `dep ensure`.

# Running the agents

```
cd multicluster/cmd/mc-agent/
export KUBECONFIG=~/.bluemix/plugins/container-service/clusters/istio-test-paid/kube-config-dal13-istio-test-paid.yml
go run main.go -configJson ../../pkg/test/mc-agent/cluster_a.json &
go run main.go -configJson ../../pkg/test/mc-agent/cluster_b.json
```
