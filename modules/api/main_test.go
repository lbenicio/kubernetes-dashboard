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
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emicklei/go-restful/v3"
	"github.com/go-openapi/spec"
	"github.com/spf13/pflag"

	"k8s.io/dashboard/api/pkg/environment"
	"k8s.io/dashboard/api/pkg/handler"
	"k8s.io/dashboard/api/pkg/integration"
	integrationapi "k8s.io/dashboard/api/pkg/integration/api"
	"k8s.io/dashboard/api/pkg/integration/metric"
	"k8s.io/dashboard/client"
)

func TestConfigureMetricsProvider(t *testing.T) {
	for _, test := range []struct {
		name        string
		provider    string
		wantSidecar bool
	}{
		{name: "disabled", provider: "none"},
		{name: "sidecar", provider: "sidecar", wantSidecar: true},
		{name: "unknown falls back to sidecar", provider: "unexpected", wantSidecar: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			setAPITestFlag(t, "metrics-provider", test.provider)
			setAPITestFlag(t, "sidecar-host", "http://metrics.example.test")
			setAPITestFlag(t, "metric-client-check-period", "7")
			metrics := &recordingMetricManager{}
			configureMetricsProvider(&recordingIntegrationManager{metric: metrics})

			if test.wantSidecar {
				if metrics.sidecarHost != "http://metrics.example.test" {
					t.Errorf("configured sidecar host = %q", metrics.sidecarHost)
				}
				if metrics.enabledID != integrationapi.SidecarIntegrationID || metrics.enabledPeriod != 7 {
					t.Errorf("EnableWithRetry() = %q, %s", metrics.enabledID, metrics.enabledPeriod)
				}
			} else if metrics.sidecarHost != "" || metrics.enabledID != "" {
				t.Errorf("metrics provider disabled but sidecar configured: %#v", metrics)
			}
		})
	}
}

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

func TestEnsureAPIServerConnectionAcceptsServerVersionResponse(t *testing.T) {
	client.Init(
		client.WithMasterUrl("https://kubernetes.example.test"),
		client.WithWrapTransport(func(http.RoundTripper) http.RoundTripper {
			return apiMainRoundTripper(func(request *http.Request) (*http.Response, error) {
				if request.Method != http.MethodGet || request.URL.Path != "/version" {
					t.Errorf("Kubernetes discovery request = %s %s", request.Method, request.URL.Path)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"major":"1","minor":"37","gitVersion":"v1.37.0"}`)),
					Request:    request,
				}, nil
			})
		}),
	)
	ensureAPIServerConnectionOrDie()
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

func TestConfigureOpenAPIUsesContainerWebServices(t *testing.T) {
	container := restful.NewContainer()
	service := new(restful.WebService).Path("/api/v1")
	container.Add(service)
	configureOpenAPI(container)
	if len(container.RegisteredWebServices()) != 2 {
		t.Fatalf("registered web services = %d, want API and OpenAPI services", len(container.RegisteredWebServices()))
	}
}

func TestInitializeAPIConfiguresProxyHandlerAndCertificates(t *testing.T) {
	setAPITestFlag(t, "apiserver-host", "https://kubernetes.example.test")
	setAPITestFlag(t, "act-as-proxy", "true")
	setAPITestFlag(t, "openapi-enabled", "true")
	setAPITestFlag(t, "auto-generate-certificates", "false")
	setAPITestFlag(t, "default-cert-dir", t.TempDir())

	apiHandler, certificates := initializeAPI()
	if apiHandler == nil {
		t.Fatal("initializeAPI() returned a nil handler")
	}
	if certificates != nil {
		t.Fatalf("initializeAPI() certificates = %v; want nil when certificate files are absent", certificates)
	}
	response := httptest.NewRecorder()
	apiHandler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/apidocs.json", nil))
	if response.Code != http.StatusOK || response.Body.Len() == 0 {
		t.Fatalf("initialized OpenAPI route = %d %q", response.Code, response.Body.String())
	}
}

func TestRunWithProxyModeSelectsConfiguredAction(t *testing.T) {
	for _, test := range []struct {
		name          string
		proxy         bool
		wantInCluster bool
	}{
		{name: "in-cluster", wantInCluster: true},
		{name: "proxy", proxy: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			inClusterCalls, proxyCalls := 0, 0
			runWithProxyMode(test.proxy, func() { inClusterCalls++ }, func() { proxyCalls++ })
			if (inClusterCalls == 1) != test.wantInCluster || (proxyCalls == 1) == test.wantInCluster {
				t.Fatalf("runWithProxyMode(%t) calls = in-cluster %d, proxy %d", test.proxy, inClusterCalls, proxyCalls)
			}
		})
	}
}

func TestRunAPIRegistersRoutesAndStartsSelectedServer(t *testing.T) {
	t.Run("insecure", func(t *testing.T) {
		mux := http.NewServeMux()
		apiHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "api") })
		attachHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "attach") })
		metricsHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "metrics") })
		started := make(chan struct {
			address string
			handler http.Handler
		}, 1)
		runAPI(apiHandler, nil, mux, attachHandler, metricsHandler, ":8000", ":8443", func(address string, handler http.Handler) error {
			started <- struct {
				address string
				handler http.Handler
			}{address, handler}
			return nil
		}, func(*http.Server) error { t.Error("TLS listener used without certificates"); return nil })
		server := <-started
		if server.address != ":8000" || server.handler != mux {
			t.Fatalf("insecure listener = %q/%T", server.address, server.handler)
		}
		for _, test := range []struct{ path, want string }{{"/", "api"}, {"/api/sockjs/session", "attach"}, {"/metrics", "metrics"}} {
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
			if response.Body.String() != test.want {
				t.Errorf("GET %s body = %q, want %q", test.path, response.Body.String(), test.want)
			}
		}
	})

	t.Run("TLS", func(t *testing.T) {
		mux := http.NewServeMux()
		certificates := []tls.Certificate{{Certificate: [][]byte{{1}}}}
		started := make(chan *http.Server, 1)
		runAPI(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), certificates, mux,
			http.NotFoundHandler(), http.NotFoundHandler(), ":8000", ":8443",
			func(string, http.Handler) error {
				t.Error("HTTP listener used when certificates are present")
				return nil
			},
			func(server *http.Server) error { started <- server; return nil })
		server := <-started
		if server.Addr != ":8443" || server.Handler != mux || server.TLSConfig.MinVersion != tls.VersionTLS12 || len(server.TLSConfig.Certificates) != 1 {
			t.Fatalf("TLS server configuration = %#v", server)
		}
	})
}

type recordingIntegrationManager struct {
	integration.Manager
	metric metric.MetricManager
}

func (in *recordingIntegrationManager) Metric() metric.MetricManager { return in.metric }

type recordingMetricManager struct {
	metric.MetricManager
	sidecarHost   string
	enabledID     integrationapi.IntegrationID
	enabledPeriod time.Duration
}

func (in *recordingMetricManager) ConfigureSidecar(host string) metric.MetricManager {
	in.sidecarHost = host
	return in
}

func (in *recordingMetricManager) EnableWithRetry(id integrationapi.IntegrationID, period time.Duration) {
	in.enabledID = id
	in.enabledPeriod = period
}

func setAPITestFlag(t *testing.T, name, value string) {
	t.Helper()
	flag := pflag.Lookup(name)
	if flag == nil {
		t.Fatalf("flag %q is not registered", name)
	}
	previous := flag.Value.String()
	t.Cleanup(func() { _ = flag.Value.Set(previous) })
	if err := flag.Value.Set(value); err != nil {
		t.Fatalf("set flag %s: %v", name, err)
	}
}

type apiMainRoundTripper func(*http.Request) (*http.Response, error)

func (in apiMainRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return in(request)
}
