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

package handler_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/emicklei/go-restful/v3"
	"github.com/spf13/pflag"
	"golang.org/x/net/xsrftoken"

	"k8s.io/dashboard/api/pkg/handler"
	"k8s.io/dashboard/api/pkg/integration"
	"k8s.io/dashboard/client"
	"k8s.io/dashboard/csrf"
)

func TestMutatingHandlersRejectMalformedEntitiesWithInitializedClient(t *testing.T) {
	initializeAPITestClient(t)

	container, err := handler.CreateHTTPAPIHandler(integration.NewIntegrationManager())
	if err != nil {
		t.Fatalf("create API handler: %v", err)
	}
	for _, test := range []struct {
		name   string
		method string
		path   string
		action string
	}{
		{name: "deploy", method: http.MethodPost, path: "/api/v1/appdeployment", action: "appdeployment"},
		{name: "deploy from file", method: http.MethodPost, path: "/api/v1/appdeploymentfromfile", action: "appdeploymentfromfile"},
		{name: "validate app name", method: http.MethodPost, path: "/api/v1/appdeployment/validate/name"},
		{name: "drain node", method: http.MethodPut, path: "/api/v1/node/node-a/drain"},
		{name: "rollback deployment", method: http.MethodPut, path: "/api/v1/deployment/default/web/rollback"},
		{name: "update raw resource", method: http.MethodPut, path: "/api/v1/_raw/pod/namespace/default/name/web"},
		{name: "create namespace", method: http.MethodPost, path: "/api/v1/namespace", action: "namespace"},
		{name: "create image pull secret", method: http.MethodPost, path: "/api/v1/secret", action: "secret"},
		{name: "update replication count", method: http.MethodPost, path: "/api/v1/replicationcontroller/default/web/update/pod", action: "replicationcontroller"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader("{"))
			request.Header.Set("Content-Type", restful.MIME_JSON)
			request.Header.Set("Authorization", "Bearer test-token")
			if test.action != "" {
				request.Header.Set("X-CSRF-TOKEN", xsrftoken.Generate(csrf.Key(), "none", test.action))
			}
			response := httptest.NewRecorder()
			container.ServeHTTP(response, request)
			if response.Code != http.StatusInternalServerError {
				t.Fatalf("%s returned status %d, want %d: %s", test.name, response.Code, http.StatusInternalServerError, response.Body.String())
			}
			body := response.Body.String()
			if !strings.Contains(body, "invalid character") && !strings.Contains(body, "unexpected EOF") && !strings.Contains(body, "JSON input") {
				t.Errorf("%s response does not report malformed JSON: %s", test.name, body)
			}
		})
	}
}

func TestApplicationNameValidationHandlesExistingResources(t *testing.T) {
	for _, test := range []struct {
		name       string
		statusCode int
		existing   string
		wantValid  bool
		wantStatus int
	}{
		{name: "name is available", statusCode: http.StatusNotFound, wantValid: true, wantStatus: http.StatusOK},
		{name: "deployment already exists", statusCode: http.StatusOK, existing: "deployments", wantValid: false, wantStatus: http.StatusOK},
		{name: "Kubernetes API failure", statusCode: http.StatusInternalServerError, wantStatus: http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			initializeAPITestClientWithTransport(t, func(request *http.Request) (*http.Response, error) {
				status := test.statusCode
				if test.existing != "" && strings.Contains(request.URL.Path, "/"+test.existing+"/") {
					kind, list, name := apiResourceKind(request.URL.Path)
					if kind == "" || list {
						t.Fatalf("unexpected validation resource request path %q", request.URL.Path)
					}
					body := `{"apiVersion":"` + apiVersionForPath(request.URL.Path) + `","kind":"` + kind + `","metadata":{"name":"` + name + `","namespace":"default"}}`
					return apiResponse(request, status, body), nil
				}
				reason, code := "NotFound", http.StatusNotFound
				if status == http.StatusInternalServerError {
					reason, code = "InternalError", http.StatusInternalServerError
				}
				body := `{"apiVersion":"v1","kind":"Status","status":"Failure","message":"test response","reason":"` + reason + `","code":` + strconv.Itoa(code) + `}`
				return apiResponse(request, status, body), nil
			})
			container, err := handler.CreateHTTPAPIHandler(integration.NewIntegrationManager())
			if err != nil {
				t.Fatalf("create API handler: %v", err)
			}
			request := httptest.NewRequest(http.MethodPost, "/api/v1/appdeployment/validate/name", strings.NewReader(`{"name":"web","namespace":"default"}`))
			request.Header.Set("Authorization", "Bearer name-validation-test-token")
			request.Header.Set("Content-Type", restful.MIME_JSON)
			response := httptest.NewRecorder()
			container.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("POST name validation = %d, want %d: %s", response.Code, test.wantStatus, response.Body.String())
			}
			if test.wantStatus != http.StatusOK {
				return
			}
			var result struct {
				Valid bool `json:"valid"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatalf("decode name validation response: %v", err)
			}
			if result.Valid != test.wantValid {
				t.Errorf("name validity = %t, want %t", result.Valid, test.wantValid)
			}
		})
	}
}

func TestResourceListHandlersReturnKubernetesResults(t *testing.T) {
	initializeAPITestClient(t)
	container, err := handler.CreateHTTPAPIHandler(integration.NewIntegrationManager())
	if err != nil {
		t.Fatalf("create API handler: %v", err)
	}
	for _, path := range []string{
		"/api/v1/namespace",
		"/api/v1/pod/default",
		"/api/v1/service/default",
		"/api/v1/event/default",
		"/api/v1/configmap/default",
		"/api/v1/secret/default",
		"/api/v1/serviceaccount/default",
	} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			request.Header.Set("Authorization", "Bearer list-handler-test-token")
			response := httptest.NewRecorder()
			container.ServeHTTP(response, request)
			var result map[string]json.RawMessage
			if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &result) != nil {
				t.Fatalf("GET %s = %d %s", path, response.Code, response.Body.String())
			}
			var errors []json.RawMessage
			if err := json.Unmarshal(result["errors"], &errors); err != nil || len(errors) != 0 {
				t.Fatalf("GET %s API errors = %s, parse error = %v", path, result["errors"], err)
			}
		})
	}
}

func TestResourceReadHandlersReturnKubernetesResults(t *testing.T) {
	initializeAPITestClientWithTransport(t, func(request *http.Request) (*http.Response, error) {
		kind, list, name := apiResourceKind(request.URL.Path)
		if kind == "" {
			return apiResponse(request, http.StatusNotFound, `{"message":"unexpected API request"}`), nil
		}
		var body string
		if list {
			body = `{"apiVersion":"` + apiVersionForPath(request.URL.Path) + `","kind":"` + kind + `List","items":[]}`
		} else {
			body = `{"apiVersion":"` + apiVersionForPath(request.URL.Path) + `","kind":"` + kind + `","metadata":{"name":"` + name + `","namespace":"default"}}`
		}
		return apiResponse(request, http.StatusOK, body), nil
	})
	container, err := handler.CreateHTTPAPIHandler(integration.NewIntegrationManager())
	if err != nil {
		t.Fatalf("create API handler: %v", err)
	}
	for _, path := range []string{
		"/api/v1/replicationcontroller",
		"/api/v1/replicaset",
		"/api/v1/pod",
		"/api/v1/deployment",
		"/api/v1/daemonset",
		"/api/v1/horizontalpodautoscaler",
		"/api/v1/job",
		"/api/v1/cronjob",
		"/api/v1/namespace",
		"/api/v1/namespace/default",
		"/api/v1/namespace/default/event",
		"/api/v1/event",
		"/api/v1/secret",
		"/api/v1/secret/default/pull-secret",
		"/api/v1/configmap",
		"/api/v1/configmap/default/app-config",
		"/api/v1/service",
		"/api/v1/service/default/web",
		"/api/v1/serviceaccount",
		"/api/v1/serviceaccount/default/builder",
		"/api/v1/serviceaccount/default/builder/secret",
		"/api/v1/serviceaccount/default/builder/imagepullsecret",
		"/api/v1/ingress",
		"/api/v1/ingress/default/web",
		"/api/v1/networkpolicy",
		"/api/v1/networkpolicy/default/restrict-egress",
		"/api/v1/statefulset",
		"/api/v1/node",
		"/api/v1/node/worker-1/event",
		"/api/v1/clusterrole",
		"/api/v1/clusterrole/view",
		"/api/v1/clusterrolebinding",
		"/api/v1/clusterrolebinding/viewers",
		"/api/v1/role/default",
		"/api/v1/role/default/viewer",
		"/api/v1/rolebinding/default",
		"/api/v1/rolebinding/default/viewers",
		"/api/v1/persistentvolume",
		"/api/v1/persistentvolume/pv-data",
		"/api/v1/persistentvolumeclaim/default",
		"/api/v1/persistentvolumeclaim/default/data",
		"/api/v1/poddisruptionbudget/default",
		"/api/v1/poddisruptionbudget/default/web",
		"/api/v1/storageclass",
		"/api/v1/storageclass/standard",
		"/api/v1/ingressclass",
		"/api/v1/ingressclass/nginx",
		"/api/v1/pod/default/web/container",
		"/api/v1/pod/default/web/event",
		"/api/v1/storageclass/standard/persistentvolume",
	} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			request.Header.Set("Authorization", "Bearer read-handler-test-token")
			response := httptest.NewRecorder()
			container.ServeHTTP(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("GET %s = %d %s", path, response.Code, response.Body.String())
			}
			var result map[string]json.RawMessage
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatalf("GET %s returned invalid JSON: %v", path, err)
			}
		})
	}
}

func TestResourceReadHandlersReportKubernetesFailures(t *testing.T) {
	initializeAPITestClientWithTransport(t, func(request *http.Request) (*http.Response, error) {
		return apiResponse(request, http.StatusInternalServerError,
			`{"apiVersion":"v1","kind":"Status","status":"Failure","message":"simulated Kubernetes API failure","reason":"InternalError","code":500}`), nil
	})
	container, err := handler.CreateHTTPAPIHandler(integration.NewIntegrationManager())
	if err != nil {
		t.Fatalf("create API handler: %v", err)
	}
	for _, path := range []string{
		"/api/v1/replicationcontroller/default/web",
		"/api/v1/replicationcontroller/default/web/pod",
		"/api/v1/replicationcontroller/default/web/event",
		"/api/v1/replicationcontroller/default/web/service",
		"/api/v1/replicaset/default/web",
		"/api/v1/replicaset/default/web/pod",
		"/api/v1/replicaset/default/web/service",
		"/api/v1/replicaset/default/web/event",
		"/api/v1/pod/default/web",
		"/api/v1/pod/default/web/container",
		"/api/v1/pod/default/web/event",
		"/api/v1/pod/default/web/persistentvolumeclaim",
		"/api/v1/deployment/default/web",
		"/api/v1/deployment/default/web/event",
		"/api/v1/deployment/default/web/oldreplicaset",
		"/api/v1/deployment/default/web/newreplicaset",
		"/api/v1/daemonset/default/web",
		"/api/v1/daemonset/default/web/pod",
		"/api/v1/daemonset/default/web/service",
		"/api/v1/daemonset/default/web/event",
		"/api/v1/horizontalpodautoscaler/default/web",
		"/api/v1/job/default/web",
		"/api/v1/job/default/web/pod",
		"/api/v1/job/default/web/event",
		"/api/v1/cronjob/default/web",
		"/api/v1/cronjob/default/web/job",
		"/api/v1/cronjob/default/web/event",
		"/api/v1/namespace/default",
		"/api/v1/secret/default/pull-secret",
		"/api/v1/configmap/default/app-config",
		"/api/v1/service/default/web",
		"/api/v1/service/default/web/event",
		"/api/v1/service/default/web/pod",
		"/api/v1/service/default/web/ingress",
		"/api/v1/serviceaccount/default/builder",
		"/api/v1/serviceaccount/default/builder/secret",
		"/api/v1/serviceaccount/default/builder/imagepullsecret",
		"/api/v1/ingress/default/web",
		"/api/v1/ingress/default/web/event",
		"/api/v1/networkpolicy/default/restrict-egress",
		"/api/v1/statefulset/default/database",
		"/api/v1/statefulset/default/database/pod",
		"/api/v1/statefulset/default/database/event",
		"/api/v1/node/worker-1",
		"/api/v1/node/worker-1/event",
		"/api/v1/node/worker-1/pod",
		"/api/v1/_raw/pod/namespace/default/name/web",
		"/api/v1/_raw/namespace/name/default",
		"/api/v1/scale/replicationcontroller/default/web",
		"/api/v1/scale/deployment/web",
		"/api/v1/clusterrole/view",
		"/api/v1/clusterrolebinding/viewers",
		"/api/v1/role/default/viewer",
		"/api/v1/rolebinding/default/viewers",
		"/api/v1/persistentvolume/pv-data",
		"/api/v1/persistentvolumeclaim/default/data",
		"/api/v1/poddisruptionbudget/default/web",
		"/api/v1/crd/widgets.example.test",
		"/api/v1/crd/default/widgets.example.test/object",
		"/api/v1/storageclass/standard",
		"/api/v1/storageclass/standard/persistentvolume",
		"/api/v1/ingressclass/nginx",
		"/api/v1/log/source/default/web/deployment",
		"/api/v1/log/default/web",
		"/api/v1/log/default/web/container",
		"/api/v1/log/file/default/web/container",
	} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			request.Header.Set("Authorization", "Bearer read-handler-error-test-token")
			response := httptest.NewRecorder()
			container.ServeHTTP(response, request)
			if response.Code == http.StatusInternalServerError {
				return
			}
			var result map[string]json.RawMessage
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatalf("GET %s = %d with invalid JSON: %s", path, response.Code, response.Body.String())
			}
			var errors []json.RawMessage
			if response.Code != http.StatusOK || json.Unmarshal(result["errors"], &errors) != nil || len(errors) == 0 {
				t.Fatalf("GET %s did not report the Kubernetes failure: status=%d body=%s", path, response.Code, response.Body.String())
			}
		})
	}
}

func apiResourceKind(path string) (string, bool, string) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	kinds := map[string]string{
		"replicationcontrollers":   "ReplicationController",
		"replicasets":              "ReplicaSet",
		"pods":                     "Pod",
		"deployments":              "Deployment",
		"daemonsets":               "DaemonSet",
		"horizontalpodautoscalers": "HorizontalPodAutoscaler",
		"jobs":                     "Job",
		"cronjobs":                 "CronJob",
		"namespaces":               "Namespace",
		"events":                   "Event",
		"resourcequotas":           "ResourceQuota",
		"limitranges":              "LimitRange",
		"secrets":                  "Secret",
		"configmaps":               "ConfigMap",
		"services":                 "Service",
		"serviceaccounts":          "ServiceAccount",
		"endpoints":                "Endpoints",
		"endpointslices":           "EndpointSlice",
		"ingresses":                "Ingress",
		"networkpolicies":          "NetworkPolicy",
		"statefulsets":             "StatefulSet",
		"nodes":                    "Node",
		"clusterroles":             "ClusterRole",
		"clusterrolebindings":      "ClusterRoleBinding",
		"roles":                    "Role",
		"rolebindings":             "RoleBinding",
		"persistentvolumes":        "PersistentVolume",
		"persistentvolumeclaims":   "PersistentVolumeClaim",
		"poddisruptionbudgets":     "PodDisruptionBudget",
		"storageclasses":           "StorageClass",
		"ingressclasses":           "IngressClass",
	}
	for i, part := range parts {
		kind, found := kinds[part]
		if !found {
			continue
		}
		if i == len(parts)-1 {
			return kind, true, ""
		}
		if i+1 == len(parts)-1 {
			return kind, false, parts[i+1]
		}
	}
	return "", false, ""
}

func apiVersionForPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) > 2 && parts[0] == "apis" {
		return parts[1] + "/" + parts[2]
	}
	if len(parts) > 1 && parts[0] == "api" {
		return parts[1]
	}
	return "v1"
}

func initializeAPITestClient(t *testing.T) {
	initializeAPITestClientWithTransport(t, func(request *http.Request) (*http.Response, error) {
		kind := ""
		switch {
		case request.URL.Path == "/api/v1/namespaces":
			kind = "NamespaceList"
		case strings.HasSuffix(request.URL.Path, "/pods"):
			kind = "PodList"
		case strings.HasSuffix(request.URL.Path, "/services"):
			kind = "ServiceList"
		case strings.HasSuffix(request.URL.Path, "/events"):
			kind = "EventList"
		case strings.HasSuffix(request.URL.Path, "/configmaps"):
			kind = "ConfigMapList"
		case strings.HasSuffix(request.URL.Path, "/secrets"):
			kind = "SecretList"
		case strings.HasSuffix(request.URL.Path, "/serviceaccounts"):
			kind = "ServiceAccountList"
		}
		status := http.StatusOK
		body := `{"apiVersion":"v1","kind":"` + kind + `","items":[]}`
		if kind == "" {
			status = http.StatusNotFound
			body = `{"message":"unexpected API request"}`
		}
		return apiResponse(request, status, body), nil
	})
}

func initializeAPITestClientWithTransport(
	t *testing.T,
	transport func(*http.Request) (*http.Response, error),
) {
	t.Helper()
	cacheFlag := pflag.Lookup("cache-enabled")
	if cacheFlag == nil {
		t.Fatal("cache-enabled flag is not registered")
	}
	previousCache := cacheFlag.Value.String()
	if err := cacheFlag.Value.Set("false"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cacheFlag.Value.Set(previousCache) })
	client.Init(
		client.WithMasterUrl("https://kubernetes.example.test"),
		client.WithWrapTransport(func(http.RoundTripper) http.RoundTripper {
			return apiRoundTripFunc(transport)
		}),
	)
}

func apiResponse(request *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    request,
	}
}

type apiRoundTripFunc func(*http.Request) (*http.Response, error)

func (in apiRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return in(request)
}
