// Licensed Materials - Property of IBM
// (C) Copyright IBM Corp. 2018. All Rights Reserved.
// US Government Users Restricted Rights - Use, duplication or
// disclosure restricted by GSA ADP Schedule Contract with IBM Corp.
// Copyright 2018 IBM Corporation

package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/ghodss/yaml"
	
	"github.com/luci/go-render/render"

	"github.com/istio-ecosystem/wharf-multicluster-sync/multicluster/pkg/config/kube/crd"
	"github.com/istio-ecosystem/wharf-multicluster-sync/multicluster/pkg/model"

	istiocrd "istio.io/istio/pilot/pkg/config/kube/crd"
	"istio.io/istio/pilot/pkg/config/memory"
	istiomodel "istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pkg/log"

	// Importing the API messages so that when resource events are fired the
	// resource will be parsed into a message object
	_ "github.com/istio-ecosystem/wharf-multicluster-sync/api/multicluster/v1alpha1"
)

// staticClusterInfo simulates the function of K8s Cluster Registry
// https://github.com/kubernetes/cluster-registry in unit tests.
type staticClusterInfo struct {
	ips   map[string]string
	ports map[string]uint32
}

var (
	filename string // input filename
	clusters string
	gengo bool
)

func main() {
	flag.Parse()

	if filename == "" {
		fmt.Printf("usage: mc-cli --filename <filename>\n")
		os.Exit(2)
	}

	in, err := os.Open(filename)
	if err != nil {
		fmt.Printf("could not open %q: %v\n", filename, err)
		os.Exit(2)
	}
	defer in.Close() // nolint: errcheck

	if gengo {
		err = convertToGo(in, os.Stdout)
	} else {
		err = readAndConvert(in, os.Stdout)
	}
	
	if err != nil {
		fmt.Printf("Error %v\n", err)
		os.Exit(3)
	}
}

func init() {
	flag.StringVar(&filename, "filename", "", "Path to YAML file containing Service Exposition Policies and Remote Service Bindings.")
	flag.StringVar(&clusters, "cluster", "", "e.g. cluster=host:port[,cluster2=host2:port2]")
	flag.BoolVar(&gengo, "gengo", false, "Generate Go code instead of YAML]")
}

// readAndConvert converts a .yaml file of ServiceExposurePolicy and RemoteServiceBinding to Istio config .yaml file
func readAndConvert(reader io.Reader, writer io.Writer) error {
	configs, err := readConfigs(reader)
	if err != nil {
		return err
	}

	ci, err := parseClusterOption(clusters)
	if err != nil {
		return err
	}
 
	store, _ := createTestConfigStore([]istiomodel.Config{})
 
	istioConfig, err := model.ConvertBindingsAndExposures(configs, ci, store)
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
	return err
}

// convertToGo converts a .yaml file of ServiceExposurePolicy and RemoteServiceBinding to
// Istio config Go source code fragment.
func convertToGo(reader io.Reader, writer io.Writer) error {
	configs, err := readConfigs(reader)
	if err != nil {
		return err
	}

	ci, err := parseClusterOption(clusters)
	if err != nil {
		return err
	}

	store, _ := createTestConfigStore([]istiomodel.Config{})

	istioConfig, err := model.ConvertBindingsAndExposures(configs, ci, store)
	if err != nil {
		return err
	}

	for _, config := range istioConfig {
		_, err = fmt.Println(render.Render(config.Spec))
		if err != nil {
			return err
		}
	}
	
	return nil
}

// parseClusterOption takes a string of the form cluster=host:port[,cluster2=host2:port2]
// and creates a staticClusterInfo
func parseClusterOption(clusters string) (model.ClusterInfo, error) {
	if clusters == "" {
		return nil, fmt.Errorf("option --cluster Cluster=host:port[,Cluster2=host:port] mandatory")
	}

	out := staticClusterInfo{
		ips:   make(map[string]string),
		ports: make(map[string]uint32),
	}

	for _, clusterExpr := range strings.Split(clusters, ",") {
		parts := strings.SplitN(clusterExpr, "=", 2)
		cluster, hostport := parts[0], parts[1]
		shost, sport, err := net.SplitHostPort(hostport)
		if err != nil {
			return nil, err
		}
		out.ips[cluster] = shost
		u, err := strconv.ParseUint(sport, 10, 32)
		if err != nil {
			return nil, err
		}
		out.ports[cluster] = uint32(u)
	}

	return &out, nil
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

	config, _, err := crd.ParseInputs(string(data))
	if err != nil {
		return nil, err
	}

	return config, nil
}

func (ci staticClusterInfo) IP(name string) string {
	out, ok := ci.ips[name]
	if ok {
		return out
	}
	return "255.255.255.255" // dummy value for unknown clusters
}

func (ci staticClusterInfo) Port(name string) uint32 {
	out, ok := ci.ports[name]
	if ok {
		return out
	}
	return 8080 // dummy value for unknown clusters
}

func createTestConfigStore(configs []istiomodel.Config) (istiomodel.ConfigStore, error) {
	out := memory.Make(istiomodel.IstioConfigTypes)
	for _, config := range configs {
		_, err := out.Create(config)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
