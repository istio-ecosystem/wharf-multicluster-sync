
To setup the test environments run:

```
./run_demo.sh
```


This scripts relies on environment variables being set in ```~/Sandbox/kube_context.sh```.
Here is the content for a set of clusters being used:

```
export  KUBECONFIG=~/.bluemix/plugins/container-service/clusters/istio-test-paid/kube-config-dal13-istio-test-paid.yml:~/.bluemix/plugins/container-service/clusters/test-multizone/kube-config-dal10-test-multizone.yml:~/.bluemix/plugins/container-service/clusters/free1/kube-config-hou02-free1.yml:~/.bluemix/plugins/container-service/clusters/istio-test-paid2/kube-config-dal13-istio-test-paid2.yml

export CLUSTER1_NAME="free1"
export CLUSTER2_NAME="test-multizone"
export ROOTCA_NAME="istio-test-paid"
export CLUSTER3_NAME="istio-test-paid2"
export CLUSTER1=$CLUSTER1_NAME
export CLUSTER2=$CLUSTER2_NAME
export CLUSTER3=$CLUSTER3_NAME

export CLUSTER1_ID="test-c1"
export CLUSTER2_ID="test-c2"
export ROOTCA_ID="root-ca"
export CLUSTER3_ID="test-c3"


export ISTIODIR=$GOPATH/src/istio.io/istio
export DEMODIR=$GOPATH/src/github.ibm.com/istio-research/multicluster-roadmap/scripts
export AGENTDIR=$GOPATH/src/github.ibm.com/istio-research/multicluster-roadmap/multicluster/cmd/mc-agent
```

In addition to the information about clusters being used, these directories are to be set:

* ISTIODIR: local istio repo
* DEMODIR: contains a clusters directpory that has cluster config files
* AGENTDIR: where pilot agents are located

There are two sample cases. For the demo, go to [samples/bookinfo](samples/bookinfo) and follow the instructions.