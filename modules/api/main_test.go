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
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/emicklei/go-restful/v3"
	"github.com/go-openapi/spec"
	"github.com/spf13/pflag"

	"k8s.io/dashboard/api/pkg/environment"
	"k8s.io/dashboard/api/pkg/handler"
	"k8s.io/dashboard/api/pkg/integration"
)

func TestEnrichOpenAPIObject(t *testing.T) {
	previous := environment.Version
	environment.Version = "v-test"
	t.Cleanup(func() { environment.Version = previous })
	swagger := &spec.Swagger{}
	enrichOpenAPIObject(swagger)
	if swagger.Info == nil || swagger.Info.Title != "Kubernetes Dashboard API" || swagger.Info.Version != "v-test" {
		t.Fatalf("OpenAPI info = %#v", swagger.Info)
	}
}

func TestConfigureOpenAPIRegistersDocumentationRoute(t *testing.T) {
	container, err := handler.CreateHTTPAPIHandler(nil)
	if err != nil {
		t.Fatalf("create API handler: %v", err)
	}
	configureOpenAPI(container)
	response := httptest.NewRecorder()
	container.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/apidocs.json", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("OpenAPI endpoint status = %d: %s", response.Code, response.Body.String())
	}
	if response.Body.Len() == 0 {
		t.Fatal("OpenAPI endpoint returned an empty document")
	}
}

func TestConfigureMetricsProviderCanBeDisabled(t *testing.T) {
	flag := pflag.Lookup("metrics-provider")
	previous := flag.Value.String()
	if err := flag.Value.Set("none"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = flag.Value.Set(previous) })
	configureMetricsProvider(integration.NewIntegrationManager())
}

func TestConfigureOpenAPIUsesContainerWebServices(t *testing.T) {
	container := restful.NewContainer()
	service := new(restful.WebService)
	service.Path("/test")
	container.Add(service)
	configureOpenAPI(container)
	if len(container.RegisteredWebServices()) != 2 {
		t.Fatalf("registered web services = %d, want API and OpenAPI services", len(container.RegisteredWebServices()))
	}
}
