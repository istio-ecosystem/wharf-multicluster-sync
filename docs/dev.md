# Developer instructions

To generate Go code for the .proto files, go to the _api_ directory and `make`.

To bring in new Go dependencies use `dep ensure`.

# Running agents locally

```
cd multicluster/cmd/mc-agent/
export KUBECONFIG=~/.bluemix/plugins/container-service/clusters/istio-test-paid/kube-config-dal13-istio-test-paid.yml
go run main.go -configJson ../../pkg/test/mc-agent/cluster_a.json &
go run main.go -configJson ../../pkg/test/mc-agent/cluster_b.json
```

# Building the agent for use on the cluster

```
cd multicluster
DOCKER_REPO=docker.io/yourname make build
DOCKER_REPO=docker.io/yourname make push
```

# Running the agent on the cluster

Note that this version uses `docker.io/mc`, if you are using a different account you must customize the file.

```
kubectl create -f sample/sample_configmap.yaml
kubectl create -f sample/deploy.yaml
```
