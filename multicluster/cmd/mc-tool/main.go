// (C) Copyright IBM Corp. 2018. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/ghodss/yaml"
	multierror "github.com/hashicorp/go-multierror"

	"github.com/luci/go-render/render"

	"github.com/istio-ecosystem/wharf-multicluster-sync/multicluster/pkg/agent"
	mccrd "github.com/istio-ecosystem/wharf-multicluster-sync/multicluster/pkg/config/kube/crd"
	mcmodel "github.com/istio-ecosystem/wharf-multicluster-sync/multicluster/pkg/model"

	istiocrd "istio.io/istio/pilot/pkg/config/kube/crd"
	"istio.io/istio/pilot/pkg/config/memory"
	istiomodel "istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pkg/log"

	kube_v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	// Importing the API messages so that when resource events are fired the
	// resource will be parsed into a message object
	_ "github.com/istio-ecosystem/wharf-multicluster-sync/api/multicluster/v1alpha1"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
)

var (
	// filename is the input YAML containing ServiceExpositionPolicy and/or RemoteServiceBinding
	filename string

	// cmFilename is the input YAML containing a ConfigMap naming clusters
	cmFilename string

	// baselineFilename is the input YAML containing starter Istio configuration
	baselineFilename string

	// clusters is deprecated; use cmFilename instead
	clusters string

	// gengo allows the tool to generate Go code (used for writing Go tests)
	gengo bool

	// mcStyle defines the output as DIRECT_INGRESS or EGRESS_INGRESS
	mcStyle string
)

func main() {
	flag.Parse()
	tool()
}

func tool() {
	if mcStyle == "" {
		// If the user did not specify --mc-style, and there is no MC_STYLE var, default to direct ingress style
		if os.Getenv(mcmodel.IstioConversionStyleKey) == "" {
			os.Setenv(mcmodel.IstioConversionStyleKey, mcmodel.DirectIngressStyle)  // nolint: errcheck
		}
	} else {
		os.Setenv(mcmodel.IstioConversionStyleKey, mcStyle)  // nolint: errcheck
	}

	if filename == "" || cmFilename == "" {
		fmt.Printf("usage: mc-tool --filename <filename> --mc-conf-filename <configmap-filename>\n")
		os.Exit(1)
	}

	in, err := os.Open(filename)
	if err != nil {
		fmt.Printf("could not open %q: %v\n", filename, err)
		os.Exit(2)
	}
	defer in.Close() // nolint: errcheck

	ci, err := parseMCConfig(cmFilename)
	if err != nil {
		fmt.Printf("could not read configuration %q: %v\n", cmFilename, err)
		os.Exit(2)
	}

	var store istiomodel.ConfigStore
	if baselineFilename != "" {
		store, err = createConfigStoreFromFile(baselineFilename)
		if err != nil {
			fmt.Printf("could not initialize Istio configuration from %q: %v\n", baselineFilename, err)
			os.Exit(2)
		}
	} else {
		store, _ = createConfigStore([]istiomodel.Config{})
	}

	// TODO load K8s services from file and merge them with generated services
	svcStore := []kube_v1.Service{}

	if gengo {
		err = convertToGo(ci, store, svcStore, in, os.Stdout)
	} else {
		err = readAndConvert(ci, store, svcStore, in, os.Stdout)
	}

	if err != nil {
		fmt.Printf("Error %v\n", err)
		os.Exit(3)
	}
}

func init() {
	flag.StringVar(&filename, "filename", "", "Path to YAML file containing Service Exposition Policies and Remote Service Bindings.")
	flag.StringVar(&cmFilename, "mc-conf-filename", "", "Path to YAML file containing Multicluster ConfigMap")
	flag.StringVar(&baselineFilename, "initial-conf-filename", "", "Path to YAML file containing baseline Istio configuration")
	flag.StringVar(&clusters, "cluster", "", "DEPRECATED; e.g. cluster=host:port[,cluster2=host2:port2]")
	flag.StringVar(&mcStyle, "mc-style", "", "Generation style: DIRECT_INGRESS|EGRESS_INGRESS")
	flag.BoolVar(&gengo, "gengo", false, "Generate Go code instead of YAML (for generating Go tests)")
}

// readAndConvert converts a .yaml file of ServiceExposurePolicy and RemoteServiceBinding to Istio config .yaml file
func readAndConvert(ci mcmodel.ClusterInfo, store istiomodel.ConfigStore, svcs []kube_v1.Service, reader io.Reader, writer io.Writer) error {
	configs, err := readConfigs(reader)
	if err != nil {
		return err
	}

	istioConfig, k8sSvcs, err := mcmodel.ConvertBindingsAndExposures2(configs, ci, store, svcs)
	if err != nil {
		return err
	}

	configDescriptor := istiomodel.ConfigDescriptor{
		istiomodel.VirtualService,
		istiomodel.Gateway,
		istiomodel.DestinationRule,
		istiomodel.ServiceEntry,
	}
	err = writeIstioYAMLOutput(configDescriptor, istioConfig, writer)
	// TODO also write k8sSvcs, if any
	_ = k8sSvcs
	return err
}

// convertToGo converts a .yaml file of ServiceExposurePolicy and RemoteServiceBinding to
// Istio config Go source code fragment.
func convertToGo(ci mcmodel.ClusterInfo, store istiomodel.ConfigStore, svcs []kube_v1.Service, reader io.Reader, writer io.Writer) error {
	configs, err := readConfigs(reader)
	if err != nil {
		return err
	}

	istioConfig, k8sSvcs, err := mcmodel.ConvertBindingsAndExposures2(configs, ci, store, svcs)
	if err != nil {
		return err
	}

	for _, config := range istioConfig {
		_, err = fmt.Println(render.Render(config.Spec))
		if err != nil {
			return err
		}
	}

	// TODO also write k8sSvcs, if any
	_ = k8sSvcs

	return nil
}

// parseMCConfig parses the kind of ConfigMap that the agents are configured with, mapping
// cluster name to host/port
func parseMCConfig(filename string) (mcmodel.ClusterInfo, error) {

	inConfig, err := os.Open(cmFilename)
	if err != nil {
		fmt.Printf("could not open %q: %v\n", cmFilename, err)
		os.Exit(2)
	}

	outConfigs := make([]kube_v1.ConfigMap, 0)

	data, err := ioutil.ReadAll(inConfig)
	if err != nil {
		return nil, err
	}

	codecs := serializer.NewCodecFactory(clientsetscheme.Scheme)
	deserializer := codecs.UniversalDeserializer()
	obj, _, err := deserializer.Decode(data, nil, nil)
	if err != nil {
		return nil, err
	}

	inConfig.Close() // nolint: errcheck

	// now use switch over the type of the object
	// and match each type-case
	switch o := obj.(type) {
	case *kube_v1.ConfigMap:
		outConfigs = append(outConfigs, *o)
	default:
		fmt.Printf("Unexpected Kubernetes type %v\n", o)
	}

	return configMapToClusterInfo(outConfigs[0])
}

func configMapToClusterInfo(cm kube_v1.ConfigMap) (mcmodel.ClusterInfo, error) {
	configYAML, ok := cm.Data["config.yaml"]
	if !ok {
		return nil, fmt.Errorf("ConfigMap does not include 'config.yaml' file") // nolint: golint
	}

	var err error
	var config agent.ClusterConfig
	err = yaml.Unmarshal([]byte(configYAML), &config)
	if err != nil {
		return nil, multierror.Prefix(err, "Could not decode config.yaml to ConfigMap")
	}
	return config, nil
}

func writeIstioYAMLOutput(descriptor istiomodel.ConfigDescriptor, configs []istiomodel.Config, writer io.Writer) error {
	for i, config := range configs {
		schema, exists := descriptor.GetByType(config.Type)
		if !exists {
			log.Errorf("Unknown kind %q for %v", istiocrd.ResourceName(config.Type), config.Name)
			continue
		}
		obj, err := istiocrd.ConvertConfig(schema, config)
		if err != nil {
			log.Errorf("Could not decode %v: %v", config.Name, err)
			continue
		}
		bytes, err := yaml.Marshal(obj)
		if err != nil {
			log.Errorf("Could not convert %v to YAML: %v", config, err)
			continue
		}
		writer.Write(bytes) // nolint: errcheck
		if i+1 < len(configs) {
			writer.Write([]byte("---\n")) // nolint: errcheck
		}
	}

	return nil
}

func readConfigs(reader io.Reader) ([]istiomodel.Config, error) {

	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	config, _, err := mccrd.ParseInputs(string(data))
	if err != nil {
		return nil, err
	}

	return config, nil
}

func createConfigStoreFromFile(fname string) (istiomodel.ConfigStore, error) {
	configs := []istiomodel.Config{}

	if fname != "" {
		reader, err := os.Open(fname)
		if err != nil {
			return nil, err
		}
		defer reader.Close() // nolint: errcheck

		configs, err = readIstioConfigs(reader)
		if err != nil {
			return nil, err
		}
	}

	return createConfigStore(configs)
}

func readIstioConfigs(reader io.Reader) ([]istiomodel.Config, error) {

	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	config, _, err := istiocrd.ParseInputs(string(data))
	if err != nil {
		return nil, err
	}

	return config, nil
}

func createConfigStore(configs []istiomodel.Config) (istiomodel.ConfigStore, error) {
	out := memory.Make(istiomodel.IstioConfigTypes)
	for _, config := range configs {
		_, err := out.Create(config)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
