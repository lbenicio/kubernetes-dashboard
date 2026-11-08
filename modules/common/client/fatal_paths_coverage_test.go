// Copyright 2017 The Kubernetes Authors.
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

package client

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"k8s.io/client-go/rest"
)

func TestClientFatalInitializationPaths(t *testing.T) {
	if fatalPath := os.Getenv("DASHBOARD_CLIENT_FATAL_PATH"); fatalPath != "" {
		switch fatalPath {
		case "session-key":
			Init(WithMasterUrl("https://kubernetes.example.test"), WithSessionEncryptionKey("invalid"))
		case "kubeconfig":
			Init(WithKubeconfig(os.Getenv("DASHBOARD_CLIENT_BAD_KUBECONFIG")))
		case "in-cluster-client":
			baseConfig = &rest.Config{Host: "%"}
			inClusterClient = nil
			InClusterClient()
		}
		t.Fatal("fatal initialization path returned")
	}

	for _, test := range []struct {
		name       string
		wantOutput string
	}{
		{name: "session-key", wantOutput: "Could not configure browser session encryption"},
		{name: "kubeconfig", wantOutput: "Could not init kubernetes client config"},
		{name: "in-cluster-client", wantOutput: "Could not init kubernetes in-cluster client"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestClientFatalInitializationPaths$")
			cmd.Env = append(os.Environ(),
				"DASHBOARD_CLIENT_FATAL_PATH="+test.name,
				"DASHBOARD_CLIENT_BAD_KUBECONFIG="+t.TempDir()+"/missing-kubeconfig",
			)
			output, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(output), test.wantOutput) {
				t.Fatalf("fatal path %q returned %v with output %q", test.name, err, output)
			}
		})
	}
}
