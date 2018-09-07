// Licensed Materials - Property of IBM
// (C) Copyright IBM Corp. 2018. All Rights Reserved.
// US Government Users Restricted Rights - Use, duplication or
// disclosure restricted by GSA ADP Schedule Contract with IBM Corp.
// Copyright 2018 IBM Corporation

package crd

import (
	"io"
	"os"
	"testing"

	istiomodel "istio.io/istio/pilot/pkg/model"

	"istio.io/istio/pilot/test/util"

	"github.ibm.com/istio-research/multicluster-roadmap/multicluster/pkg/model"
)

func TestBindingToSNIConfiguration(t *testing.T) {
	tt := []struct {
		in  string
		out string
	}{
		{in: "rshriram-demo-binding.yaml",
			out: "banix-demo-binding.yaml"},
		{in: "rshriram-demo-exposure.yaml",
			out: "banix-demo-exposure.yaml"},
		{in: "reviews-binding.yaml",
			out: "reviews-sni-binding.yaml"},
// TODO restore
//		{in: "reviews-binding-v1-only.yaml",
//			out: "reviews-sni-binding-v1-only.yaml"},
// TODO restore
//		{in: "reviews-exposure.yaml",
//			out: "reviews-sni-exposure.yaml"},
	}

	for _, tc := range tt {
		t.Run(tc.in, func(t *testing.T) {
			in, err := os.Open("../../../test/expose-binding/" + tc.in)
			if err != nil {
				t.Fatal(err)
			}
			defer in.Close() // nolint: errcheck

			outFilename := "../../../test/istio-expose-binding/" + tc.out
			out, err := os.Create(outFilename)
			if err != nil {
				t.Fatal(err)
			}
			defer out.Close() // nolint: errcheck

			if err := readAndConvertSNI(in, out); err != nil {
				t.Fatalf("Unexpected error converting configs: %v", err)
			}

			util.CompareYAML(outFilename, t)
		})
	}
}

// readAndConvertSNI converts a .yaml file of ServiceExposurePolicy and RemoteServiceBinding to Istio config .yaml file
func readAndConvertSNI(reader io.Reader, writer io.Writer) error {
	configs, err := readConfigs(reader)
	if err != nil {
		return err
	}

	ci := debugClusterInfo{
		ips: map[string]string{
			"clusterC": "127.0.0.1",
			"cluster2": "127.0.0.1",
		},
		ports: map[string]uint32{
			"clusterC": 80,
			"cluster2": 80,
		},
	}
	istioConfig, err := model.ConvertBindingsAndExposuresSNI(configs, ci)
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
	if err != nil {
		return err
	}

	return nil
}
