// Copyright 2017 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package client

import (
	"net/http/httptest"
	"testing"

	"github.com/spf13/pflag"
	"k8s.io/client-go/rest"
)

func TestClientFactoriesBuildClientsWithoutConnectingToTheCluster(t *testing.T) {
	previousConfig, previousInClusterClient := baseConfig, inClusterClient
	baseConfig = &rest.Config{Host: "https://kubernetes.example.test"}
	inClusterClient = nil
	t.Cleanup(func() {
		baseConfig = previousConfig
		inClusterClient = previousInClusterClient
	})

	cacheFlag := pflag.Lookup("cache-enabled")
	previousCache := cacheFlag.Value.String()
	if err := cacheFlag.Value.Set("false"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cacheFlag.Value.Set(previousCache) })

	request := httptest.NewRequest("GET", "https://dashboard.example.test", nil)
	SetAuthorizationHeader(request, "test-token")
	if got, err := Client(request); err != nil || got == nil {
		t.Fatalf("Client() = %v, %v", got, err)
	}
	if got, err := APIExtensionsClient(request); err != nil || got == nil {
		t.Fatalf("APIExtensionsClient() = %v, %v", got, err)
	}
	if got, err := Config(request); err != nil || got.Host != baseConfig.Host || got.BearerToken != "test-token" {
		t.Fatalf("Config() = %#v, %v", got, err)
	}
	if got := InClusterClient(); got == nil {
		t.Fatal("InClusterClient() returned nil for an initialized client")
	}
	if got, err := RestClientForHost("https://other-kubernetes.example.test"); err != nil || got == nil {
		t.Fatalf("RestClientForHost() = %v, %v", got, err)
	}
	if CanI(httptest.NewRequest("GET", "https://dashboard.example.test", nil), nil) {
		t.Fatal("CanI() granted a request without an authorization header")
	}
	if got := setConfigRateLimitDefaults(nil); got != nil {
		t.Fatalf("setConfigRateLimitDefaults(nil) = %v", got)
	}
}
