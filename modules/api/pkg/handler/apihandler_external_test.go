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

func TestExecShellRequiresAnAuthenticatedClient(t *testing.T) {
	initializeAPITestClient(t)
	container, err := handler.CreateHTTPAPIHandler(integration.NewIntegrationManager())
	if err != nil {
		t.Fatalf("create API handler: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/pod/default/web/shell/main", nil)
	response := httptest.NewRecorder()
	container.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated shell request status = %d; want %d: %s", response.Code, http.StatusUnauthorized, response.Body.String())
	}
}

func TestDeployFromFileHandlerReturnsCreatedResponseAndPropagatesFailures(t *testing.T) {
	manifest := "apiVersion: v1\nkind: Pod\nmetadata:\n  name: web\n"
	for _, test := range []struct {
		name       string
		manifest   string
		createCode int
		wantStatus int
		wantError  string
	}{
		{name: "created namespaced resource", manifest: manifest, createCode: http.StatusCreated, wantStatus: http.StatusCreated},
		{name: "unknown resource kind", manifest: strings.Replace(manifest, "kind: Pod", "kind: Unknown", 1), createCode: http.StatusCreated, wantStatus: http.StatusInternalServerError, wantError: "unknown resource kind: Unknown"},
		{name: "Kubernetes create failure", manifest: manifest, createCode: http.StatusForbidden, wantStatus: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			initializeAPITestClientWithTransport(t, func(request *http.Request) (*http.Response, error) {
				if request.Method == http.MethodGet && request.URL.Path == "/api/v1" {
					body := `{"kind":"APIResourceList","apiVersion":"v1","resources":[{"name":"pods","singularName":"pod","namespaced":true,"kind":"Pod"}]}`
					return apiResponse(request, http.StatusOK, body), nil
				}
				if request.Method == http.MethodPost && request.URL.Path == "/api/v1/namespaces/team-a/pods" {
					if test.createCode != http.StatusCreated {
						body := `{"kind":"Status","apiVersion":"v1","status":"Failure","message":"create denied","reason":"Forbidden","code":403}`
						return apiResponse(request, test.createCode, body), nil
					}
					body := `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"web","namespace":"team-a"}}`
					return apiResponse(request, test.createCode, body), nil
				}
				return apiResponse(request, http.StatusNotFound, `{"kind":"Status","apiVersion":"v1","status":"Failure","code":404}`), nil
			})

			container, err := handler.CreateHTTPAPIHandler(integration.NewIntegrationManager())
			if err != nil {
				t.Fatalf("create API handler: %v", err)
			}
			body, err := json.Marshal(map[string]string{"name": "manifest.yaml", "namespace": "team-a", "content": test.manifest})
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodPost, "/api/v1/appdeploymentfromfile", strings.NewReader(string(body)))
			request.Header.Set("Authorization", "Bearer deploy-file-test-token")
			request.Header.Set("Content-Type", restful.MIME_JSON)
			request.Header.Set("X-CSRF-TOKEN", xsrftoken.Generate(csrf.Key(), "none", "appdeploymentfromfile"))
			response := httptest.NewRecorder()
			container.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("POST appdeploymentfromfile = %d, want %d: %s", response.Code, test.wantStatus, response.Body.String())
			}
			if test.wantError != "" && !strings.Contains(response.Body.String(), test.wantError) {
				t.Errorf("response = %s; want error %q", response.Body.String(), test.wantError)
			}
			if test.wantStatus == http.StatusCreated {
				var result struct {
					Name    string `json:"name"`
					Content string `json:"content"`
					Error   string `json:"error"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
					t.Fatalf("decode deploy response: %v", err)
				}
				if result.Name != "manifest.yaml" || result.Content != test.manifest || result.Error != "" {
					t.Errorf("deploy response = %#v", result)
				}
			}
		})
	}
}

func TestDeployHandlerCreatesDeploymentAndReturnsKubernetesErrors(t *testing.T) {
	for _, test := range []struct {
		name       string
		createCode int
		wantStatus int
	}{
		{name: "deployment created", createCode: http.StatusCreated, wantStatus: http.StatusCreated},
		{name: "Kubernetes create rejected", createCode: http.StatusForbidden, wantStatus: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			initializeAPITestClientWithTransport(t, func(request *http.Request) (*http.Response, error) {
				if request.Method == http.MethodPost && request.URL.Path == "/apis/apps/v1/namespaces/team-a/deployments" {
					if test.createCode != http.StatusCreated {
						body := `{"kind":"Status","apiVersion":"v1","status":"Failure","message":"create denied","reason":"Forbidden","code":403}`
						return apiResponse(request, test.createCode, body), nil
					}
					body := `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"web","namespace":"team-a"}}`
					return apiResponse(request, test.createCode, body), nil
				}
				return apiResponse(request, http.StatusNotFound, `{"kind":"Status","apiVersion":"v1","status":"Failure","code":404}`), nil
			})

			container, err := handler.CreateHTTPAPIHandler(integration.NewIntegrationManager())
			if err != nil {
				t.Fatalf("create API handler: %v", err)
			}
			spec := map[string]any{"name": "web", "containerImage": "example/web:v1", "namespace": "team-a", "replicas": 2, "labels": []map[string]string{{"key": "app", "value": "web"}}}
			body, err := json.Marshal(spec)
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodPost, "/api/v1/appdeployment", strings.NewReader(string(body)))
			request.Header.Set("Authorization", "Bearer deploy-test-token")
			request.Header.Set("Content-Type", restful.MIME_JSON)
			request.Header.Set("X-CSRF-TOKEN", xsrftoken.Generate(csrf.Key(), "none", "appdeployment"))
			response := httptest.NewRecorder()
			container.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("POST appdeployment = %d, want %d: %s", response.Code, test.wantStatus, response.Body.String())
			}
			if test.wantStatus == http.StatusCreated {
				var result map[string]any
				if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
					t.Fatalf("decode appdeployment response: %v", err)
				}
				if result["name"] != "web" || result["namespace"] != "team-a" || result["containerImage"] != "example/web:v1" {
					t.Errorf("appdeployment response = %#v", result)
				}
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
		switch request.URL.Path {
		case "/apis/apps/v1/namespaces/default/statefulsets/database":
			return apiResponse(request, http.StatusOK, `{"apiVersion":"apps/v1","kind":"StatefulSet","metadata":{"name":"database","namespace":"default"},"spec":{"selector":{"matchLabels":{"app":"database"}},"template":{"metadata":{"labels":{"app":"database"}},"spec":{"containers":[{"name":"database","image":"example/database"}]}}}}`), nil
		case "/apis/apps/v1/namespaces/default/daemonsets/agents":
			return apiResponse(request, http.StatusOK, `{"apiVersion":"apps/v1","kind":"DaemonSet","metadata":{"name":"agents","namespace":"default"},"spec":{"selector":{"matchLabels":{"app":"agents"}},"template":{"metadata":{"labels":{"app":"agents"}},"spec":{"containers":[{"name":"agent","image":"example/agent"}]}}}}`), nil
		case "/apis/apps/v1/namespaces/default/replicasets/web":
			return apiResponse(request, http.StatusOK, `{"apiVersion":"apps/v1","kind":"ReplicaSet","metadata":{"name":"web","namespace":"default"},"spec":{"selector":{"matchLabels":{"app":"web"}},"template":{"metadata":{"labels":{"app":"web"}},"spec":{"containers":[{"name":"web","image":"nginx"}]}}}}`), nil
		case "/api/v1/namespaces/default/replicationcontrollers/web":
			return apiResponse(request, http.StatusOK, `{"apiVersion":"v1","kind":"ReplicationController","metadata":{"name":"web","namespace":"default"},"spec":{"selector":{"app":"web"},"template":{"metadata":{"labels":{"app":"web"}},"spec":{"containers":[{"name":"web","image":"nginx"}]}}}}`), nil
		case "/apis/batch/v1/namespaces/default/jobs/web":
			return apiResponse(request, http.StatusOK, `{"apiVersion":"batch/v1","kind":"Job","metadata":{"name":"web","namespace":"default"},"spec":{"selector":{"matchLabels":{"job":"web"}},"template":{"metadata":{"labels":{"job":"web"}},"spec":{"containers":[{"name":"web","image":"busybox"}],"restartPolicy":"Never"}}}}`), nil
		case "/apis/apps/v1/namespaces/default/deployments/web":
			return apiResponse(request, http.StatusOK, `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"web","namespace":"default"},"spec":{"selector":{"matchLabels":{"app":"web"}},"template":{"metadata":{"labels":{"app":"web"}},"spec":{"containers":[{"name":"web","image":"nginx"}]}}}}`), nil
		}
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
		"/api/v1/replicaset/default/web",
		"/api/v1/pod",
		"/api/v1/pod/default/web",
		"/api/v1/pod/default/web/persistentvolumeclaim",
		"/api/v1/deployment",
		"/api/v1/daemonset",
		"/api/v1/horizontalpodautoscaler",
		"/api/v1/horizontalpodautoscaler/default/web",
		"/api/v1/deployment/default/web/horizontalpodautoscaler",
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
		"/api/v1/statefulset/default/database",
		"/api/v1/statefulset/default/database/pod",
		"/api/v1/statefulset/default/database/event",
		"/api/v1/node",
		"/api/v1/node/worker-1",
		"/api/v1/node/worker-1/event",
		"/api/v1/node/worker-1/pod",
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
		"/api/v1/service/default/web/event",
		"/api/v1/service/default/web/pod",
		"/api/v1/service/default/web/ingress",
		"/api/v1/ingress/default/web/event",
		"/api/v1/deployment/default/web/event",
		"/api/v1/deployment/default/web/oldreplicaset",
		"/api/v1/deployment/default/web/newreplicaset",
		"/api/v1/replicaset/default/web/event",
		"/api/v1/replicaset/default/web/service",
		"/api/v1/replicationcontroller/default/web",
		"/api/v1/replicationcontroller/default/web/pod",
		"/api/v1/replicationcontroller/default/web/event",
		"/api/v1/replicationcontroller/default/web/service",
		"/api/v1/daemonset/default/agents",
		"/api/v1/daemonset/default/agents/pod",
		"/api/v1/daemonset/default/agents/service",
		"/api/v1/daemonset/default/agents/event",
		"/api/v1/job/default/web",
		"/api/v1/job/default/web/pod",
		"/api/v1/job/default/web/event",
		"/api/v1/cronjob/default/backup",
		"/api/v1/cronjob/default/backup/job",
		"/api/v1/cronjob/default/backup/event",
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

func TestCustomResourceReadHandlersReturnKubernetesResults(t *testing.T) {
	crd := `{"apiVersion":"apiextensions.k8s.io/v1","kind":"CustomResourceDefinition","metadata":{"name":"widgets.example.test"},"spec":{"group":"example.test","names":{"plural":"widgets","singular":"widget","kind":"Widget","listKind":"WidgetList"},"scope":"Namespaced","versions":[{"name":"v1","served":true,"storage":true,"subresources":{"scale":{"specReplicasPath":".spec.replicas","statusReplicasPath":".status.replicas"},"status":{}}}]},"status":{"acceptedNames":{"plural":"widgets","singular":"widget","kind":"Widget","listKind":"WidgetList"},"conditions":[{"type":"Established","status":"True"}]}}`
	customResource := `{"apiVersion":"example.test/v1","kind":"Widget","metadata":{"name":"sample","namespace":"default"}}`
	initializeAPITestClientWithTransport(t, func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.Path == "/api":
			return apiResponse(request, http.StatusOK, `{"kind":"APIVersions","versions":["v1"]}`), nil
		case request.URL.Path == "/apis":
			return apiResponse(request, http.StatusOK, `{"kind":"APIGroupList","groups":[{"name":"apiextensions.k8s.io","versions":[{"groupVersion":"apiextensions.k8s.io/v1","version":"v1"}],"preferredVersion":{"groupVersion":"apiextensions.k8s.io/v1","version":"v1"}},{"name":"authorization.k8s.io","versions":[{"groupVersion":"authorization.k8s.io/v1","version":"v1"}],"preferredVersion":{"groupVersion":"authorization.k8s.io/v1","version":"v1"}},{"name":"example.test","versions":[{"groupVersion":"example.test/v1","version":"v1"}],"preferredVersion":{"groupVersion":"example.test/v1","version":"v1"}}]}`), nil
		case request.URL.Path == "/api/v1":
			return apiResponse(request, http.StatusOK, `{"kind":"APIResourceList","groupVersion":"v1","resources":[{"name":"events","singularName":"event","namespaced":true,"kind":"Event","verbs":["get","list"]}]}`), nil
		case request.URL.Path == "/apis/apiextensions.k8s.io/v1":
			return apiResponse(request, http.StatusOK, `{"kind":"APIResourceList","groupVersion":"apiextensions.k8s.io/v1","resources":[{"name":"customresourcedefinitions","singularName":"customresourcedefinition","namespaced":false,"kind":"CustomResourceDefinition","verbs":["get","list"]}]}`), nil
		case request.URL.Path == "/apis/authorization.k8s.io/v1":
			return apiResponse(request, http.StatusOK, `{"kind":"APIResourceList","groupVersion":"authorization.k8s.io/v1","resources":[{"name":"selfsubjectaccessreviews","singularName":"selfsubjectaccessreview","namespaced":false,"kind":"SelfSubjectAccessReview","verbs":["create"]}]}`), nil
		case request.URL.Path == "/apis/example.test/v1":
			return apiResponse(request, http.StatusOK, `{"kind":"APIResourceList","groupVersion":"example.test/v1","resources":[{"name":"widgets","singularName":"widget","namespaced":true,"kind":"Widget","verbs":["get","list"]}]}`), nil
		case request.URL.Path == "/apis/apiextensions.k8s.io/v1/customresourcedefinitions":
			return apiResponse(request, http.StatusOK, `{"apiVersion":"apiextensions.k8s.io/v1","kind":"CustomResourceDefinitionList","items":[`+crd+`]}`), nil
		case strings.HasPrefix(request.URL.Path, "/apis/apiextensions.k8s.io/v1/customresourcedefinitions/"):
			return apiResponse(request, http.StatusOK, crd), nil
		case request.URL.Path == "/apis/example.test/v1/widgets" || request.URL.Path == "/apis/example.test/v1/namespaces/default/widgets":
			return apiResponse(request, http.StatusOK, `{"apiVersion":"example.test/v1","kind":"WidgetList","items":[`+customResource+`]}`), nil
		case request.URL.Path == "/apis/example.test/v1/namespaces/default/widgets/sample":
			return apiResponse(request, http.StatusOK, customResource), nil
		case request.URL.Path == "/api/v1/namespaces/default/events":
			return apiResponse(request, http.StatusOK, `{"apiVersion":"v1","kind":"EventList","items":[]}`), nil
		default:
			return apiResponse(request, http.StatusNotFound, `{"message":"unexpected request"}`), nil
		}
	})
	container, err := handler.CreateHTTPAPIHandler(integration.NewIntegrationManager())
	if err != nil {
		t.Fatalf("create API handler: %v", err)
	}
	for _, path := range []string{
		"/api/v1/crd",
		"/api/v1/crd/widgets.example.test",
		"/api/v1/crd/default/widgets.example.test/object",
		"/api/v1/crd/default/widgets.example.test/sample",
		"/api/v1/crd/default/widgets.example.test/sample/event",
	} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			request.Header.Set("Authorization", "Bearer custom-resource-read-test-token")
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

func TestNamespaceEventHandlerReportsKubernetesListFailure(t *testing.T) {
	initializeAPITestClientWithTransport(t, func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/v1/namespaces/default/events" {
			return apiResponse(request, http.StatusNotFound, `{"kind":"Status","status":"Failure","code":404}`), nil
		}
		return apiResponse(request, http.StatusInternalServerError, `{"kind":"Status","status":"Failure","reason":"InternalError","message":"event storage unavailable","code":500}`), nil
	})
	container, err := handler.CreateHTTPAPIHandler(integration.NewIntegrationManager())
	if err != nil {
		t.Fatalf("create API handler: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/namespace/default/event", nil)
	request.Header.Set("Authorization", "Bearer namespace-event-error-test-token")
	response := httptest.NewRecorder()
	container.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("namespace event status = %d; want %d: %s", response.Code, http.StatusInternalServerError, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "event storage unavailable") {
		t.Fatalf("namespace event error response = %q", response.Body.String())
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
		"/api/v1/replicationcontroller",
		"/api/v1/replicaset",
		"/api/v1/pod",
		"/api/v1/deployment",
		"/api/v1/daemonset",
		"/api/v1/horizontalpodautoscaler",
		"/api/v1/job",
		"/api/v1/cronjob",
		"/api/v1/namespace",
		"/api/v1/event",
		"/api/v1/secret",
		"/api/v1/configmap",
		"/api/v1/service",
		"/api/v1/serviceaccount",
		"/api/v1/ingress",
		"/api/v1/networkpolicy",
		"/api/v1/statefulset",
		"/api/v1/node",
		"/api/v1/clusterrole",
		"/api/v1/clusterrolebinding",
		"/api/v1/role/default",
		"/api/v1/rolebinding/default",
		"/api/v1/persistentvolume",
		"/api/v1/persistentvolumeclaim/default",
		"/api/v1/poddisruptionbudget/default",
		"/api/v1/storageclass",
		"/api/v1/ingressclass",
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
		"/api/v1/statefulset/default/database",
		"/api/v1/statefulset/default/database/pod",
		"/api/v1/statefulset/default/database/event",
		"/api/v1/job/default/web",
		"/api/v1/job/default/web/pod",
		"/api/v1/job/default/web/event",
		"/api/v1/cronjob/default/backup",
		"/api/v1/cronjob/default/backup/job",
		"/api/v1/cronjob/default/backup/event",
		"/api/v1/node/worker-1",
		"/api/v1/node/worker-1/event",
		"/api/v1/node/worker-1/pod",
		"/api/v1/ingress/default/web/event",
		"/api/v1/service/default/web/event",
		"/api/v1/service/default/web/pod",
		"/api/v1/service/default/web/ingress",
		"/api/v1/horizontalpodautoscaler/default/web",
		"/api/v1/deployment/default/web/horizontalpodautoscaler",
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
		"/api/v1/crd",
		"/api/v1/crd/default/widgets.example.test/object",
		"/api/v1/crd/default/widgets.example.test/sample",
		"/api/v1/crd/default/widgets.example.test/sample/event",
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

func TestResourceReadHandlersRejectRequestsWithoutCredentials(t *testing.T) {
	initializeAPITestClientWithTransport(t, func(request *http.Request) (*http.Response, error) {
		t.Errorf("unauthenticated request reached Kubernetes API: %s %s", request.Method, request.URL.Path)
		return apiResponse(request, http.StatusOK, `{}`), nil
	})
	container, err := handler.CreateHTTPAPIHandler(integration.NewIntegrationManager())
	if err != nil {
		t.Fatalf("create API handler: %v", err)
	}
	paths := []string{
		"/api/v1/replicationcontroller",
		"/api/v1/replicaset",
		"/api/v1/pod",
		"/api/v1/deployment",
		"/api/v1/daemonset",
		"/api/v1/horizontalpodautoscaler",
		"/api/v1/deployment/default/web/horizontalpodautoscaler",
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
		"/api/v1/crd",
		"/api/v1/crd/widgets.example.test",
		"/api/v1/crd/default/widgets.example.test/object",
		"/api/v1/crd/default/widgets.example.test/sample",
		"/api/v1/crd/default/widgets.example.test/sample/event",
		"/api/v1/log/source/default/web/deployment",
		"/api/v1/log/default/pod/web",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			response := httptest.NewRecorder()
			container.ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), "MSG_LOGIN_UNAUTHORIZED_ERROR") {
				t.Fatalf("GET %s without credentials = %d %s, want unauthorized", path, response.Code, response.Body.String())
			}
		})
	}
}

func TestMutationHandlersPropagateKubernetesFailures(t *testing.T) {
	initializeAPITestClientWithTransport(t, func(request *http.Request) (*http.Response, error) {
		return apiResponse(request, http.StatusInternalServerError, `{"apiVersion":"v1","kind":"Status","status":"Failure","message":"test failure","reason":"InternalError","code":500}`), nil
	})
	container, err := handler.CreateHTTPAPIHandler(integration.NewIntegrationManager())
	if err != nil {
		t.Fatalf("create API handler: %v", err)
	}

	for _, test := range []struct {
		name   string
		method string
		path   string
		body   string
		action string
	}{
		{name: "deploy application", method: http.MethodPost, path: "/api/v1/appdeployment", body: `{"name":"web","containerImage":"nginx:1.27","replicas":1,"namespace":"default"}`, action: "appdeployment"},
		{name: "apply manifest", method: http.MethodPost, path: "/api/v1/appdeploymentfromfile", body: `{"name":"manifest.yaml","namespace":"default","content":"apiVersion: v1\\nkind: ConfigMap\\nmetadata:\\n  name: sample\\n  namespace: default\\n","validate":false}`, action: "appdeploymentfromfile"},
		{name: "pause deployment", method: http.MethodPut, path: "/api/v1/deployment/default/web/pause"},
		{name: "rollback deployment", method: http.MethodPut, path: "/api/v1/deployment/default/web/rollback", body: `{"revision":"1"}`},
		{name: "restart deployment", method: http.MethodPut, path: "/api/v1/deployment/default/web/restart"},
		{name: "resume deployment", method: http.MethodPut, path: "/api/v1/deployment/default/web/resume"},
		{name: "restart daemonset", method: http.MethodPut, path: "/api/v1/daemonset/default/agents/restart"},
		{name: "restart statefulset", method: http.MethodPut, path: "/api/v1/statefulset/default/database/restart"},
		{name: "trigger cronjob", method: http.MethodPut, path: "/api/v1/cronjob/default/backup/trigger"},
		{name: "scale deployment", method: http.MethodPut, path: "/api/v1/scale/deployment/default/web?scaleBy=2"},
		{name: "read replica count", method: http.MethodGet, path: "/api/v1/scale/deployment/default/web"},
		{name: "update raw resource", method: http.MethodPut, path: "/api/v1/_raw/pod/namespace/default/name/web", body: `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"web","namespace":"default"}}`},
		{name: "reject malformed raw resource", method: http.MethodPut, path: "/api/v1/_raw/pod/namespace/default/name/web", body: `{`},
		{name: "delete raw resource", method: http.MethodDelete, path: "/api/v1/_raw/pod/namespace/default/name/web"},
		{name: "create namespace", method: http.MethodPost, path: "/api/v1/namespace", body: `{"name":"test-namespace"}`, action: "namespace"},
		{name: "create image pull secret", method: http.MethodPost, path: "/api/v1/secret", body: `{"name":"registry","namespace":"default","data":"e30="}`, action: "secret"},
		{name: "scale replication controller", method: http.MethodPost, path: "/api/v1/replicationcontroller/default/web/update/pod", body: `{"replicas":2}`, action: "replicationcontroller"},
		{name: "drain node", method: http.MethodPut, path: "/api/v1/node/node-a/drain", body: `{}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			request.Header.Set("Authorization", "Bearer mutation-test-token")
			request.Header.Set("Content-Type", restful.MIME_JSON)
			if test.action != "" {
				request.Header.Set("X-CSRF-TOKEN", xsrftoken.Generate(csrf.Key(), "none", test.action))
			}
			response := httptest.NewRecorder()
			container.ServeHTTP(response, request)
			if response.Code != http.StatusInternalServerError {
				t.Fatalf("%s returned %d, want %d: %s", test.name, response.Code, http.StatusInternalServerError, response.Body.String())
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
