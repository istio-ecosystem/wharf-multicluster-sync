// Licensed Materials - Property of IBM
// (C) Copyright IBM Corp. 2018. All Rights Reserved.
// US Government Users Restricted Rights - Use, duplication or
// disclosure restricted by GSA ADP Schedule Contract with IBM Corp.
// Copyright 2018 IBM Corporation

package crd

import (
	"os"
	"testing"

	"istio.io/istio/pilot/test/util"
)

func TestBindingToConfigurationEgressIngress(t *testing.T) {
	tt := []struct {
		config string // Config of binding cluster
		in     string
		out    string
	}{
		{config: "cluster1.json",
			in:  "sample-binding.yaml",
			out: "sample-binding.yaml"},
		{config: "cluster_a.json",
			in:  "sample-exposure.yaml",
			out: "sample-exposure.yaml"},
		{config: "cluster1.json",
			in:  "rshriram-demo-binding.yaml",
			out: "rshriram-demo-binding.yaml"},
		{config: "cluster_a.json",
			in:  "rshriram-demo-exposure.yaml",
			out: "rshriram-demo-exposure.yaml"},
	}

	for _, tc := range tt {
		t.Run(tc.in, func(t *testing.T) {
			clusterConfig, err := loadConfig("../../../test/mc-agent/" + tc.config)
			if err != nil {
				t.Fatal(err)
			}

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

			if err := readAndConvert(in, out, clusterConfig); err != nil {
				t.Fatalf("Unexpected error converting configs: %v", err)
			}

			util.CompareYAML(outFilename, t)
		})
	}
}
