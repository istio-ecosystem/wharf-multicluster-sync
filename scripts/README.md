
To setup the test environments run:

```
./run_demo.sh
```


This scripts relies on environment variables being set in ```~/Sandbox/kube_context.sh```.
Here is the content for a set of clusters being used:

```
export  KUBECONFIG=~/Sandbox/mb1_admin.conf:/Users/mb/.bluemix/plugins/container-service/clusters/istio-test-paid/kube-config-dal13-istio-test-paid.yml:/Users/mb/.bluemix/plugins/container-service/clusters/test-multizone/kube-config-dal10-test-multizone.yml:/Users/mb/.bluemix/plugins/container-service/clusters/free1/kube-config-hou02-free1.yml:/Users/mb/.bluemix/plugins/container-service/clusters/istio-test-paid2/kube-config-dal13-istio-test-paid2.yml

export CLUSTER1_NAME="free1"
export CLUSTER2_NAME="test-multizone"
export ROOTCA_NAME="istio-test-paid"
export CLUSTER3_NAME="istio-test-paid2"

export CLUSTER1_ID="test-c1"
export CLUSTER2_ID="test-c2"
export ROOTCA_ID="root-ca"
export CLUSTER3_ID="test-c3"


export ISTIODIR="/Users/mb/Repos/istio-1.0.2"
export DEMODIR="/Users/mb/Documents/Repos/istio_federation_demo/demo"
export AGENTDIR="/Users/mb/go/src/github.ibm.com/istio-research/multicluster-roadmap/multicluster/cmd/mc-agent"
```

In addition to the information about clusters being used, these directories are to be set:

* ISTIODIR: local istio repo
* DEMODIR: contains a clusters directpory that has cluster config files
* AGENTDIR: where pilot agents are located

There are two sample cases. For the demo, go to ```samples/bookinfo``` and follow the instructions.