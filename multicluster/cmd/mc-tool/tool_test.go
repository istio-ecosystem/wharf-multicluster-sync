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
	"os"
	"testing"

	"istio.io/istio/pilot/test/util"
)

// TestServiceToBinding tests agent.exposedServicesToBinding()
func TestTool(t *testing.T) {
	tt := []struct {
		filename   string // Service Exposition Policy or Remote Service Binding
		cmFilename string // Config of cluster
		out        string // Filename of .golden file (to be used for comparison in future)
	}{
		{filename: "reviews-binding.yaml",
			cmFilename: "cluster1.yaml",
			out:        "reviews-binding.yaml"},
		{filename: "reviews-exposure.yaml",
			cmFilename: "cluster1.yaml",
			out:        "reviews-exposure.yaml"},
	}

	for _, tc := range tt {
		t.Run(tc.out, func(t *testing.T) {
			// Set the globals the CLI tool uses
			filename = "../../pkg/test/expose-binding/" + tc.filename
			cmFilename = "../../pkg/test/mc-agent/" + tc.cmFilename

			outFilename := "../../pkg/test/cli-tool/" + tc.out
			outFile, err := os.Create(outFilename)
			if err != nil {
				t.Errorf("can't create %q: %v", outFilename, err)
			}

			origStdout := os.Stdout // Capture stdout
			os.Stdout = outFile

			tool()

			outFile.Close() // Restore stdout // nolint: errcheck
			os.Stdout = origStdout

			util.CompareYAML(outFilename, t)
		})
	}
}
