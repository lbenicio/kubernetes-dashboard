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

package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/emicklei/go-restful/v3"
	"github.com/spf13/pflag"
	"golang.org/x/net/xsrftoken"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"k8s.io/dashboard/api/pkg/args"
	"k8s.io/dashboard/api/pkg/validation"
	"k8s.io/dashboard/csrf"
)

func TestCreateHTTPAPIHandler(t *testing.T) {
	_, err := CreateHTTPAPIHandler(nil)
	if err != nil {
		t.Fatal("CreateHTTPAPIHandler() cannot create HTTP API handler")
	}
}

func TestParseNamespacePathParameterTrimsAndDropsEmptyEntries(t *testing.T) {
	ws := new(restful.WebService)
	ws.Path("/probe")
	container := restful.NewContainer()
	container.Add(ws.Route(ws.GET("/{namespace}").To(func(request *restful.Request, response *restful.Response) {
		query := parseNamespacePathParameter(request)
		if !query.Matches("default") || !query.Matches("team-a") || query.Matches(" ") {
			t.Errorf("namespace query did not trim/drop values")
		}
		response.WriteHeader(http.StatusOK)
	})))
	request := httptest.NewRequest(http.MethodGet, "/probe/default,%20%20,team-a%20", nil)
	response := httptest.NewRecorder()
	container.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("namespace probe returned %d: %s", response.Code, response.Body.String())
	}
}

func TestCsrfTokenEndpoint(t *testing.T) {
	response := serveAPIRequest(t, http.MethodGet, "/api/v1/csrftoken/login", "")
	if response.Code != http.StatusOK {
		t.Fatalf("GET csrf token status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}

	var result csrf.Response
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode csrf token response: %v", err)
	}
	if !xsrftoken.Valid(result.Token, csrf.Key(), "none", "login") {
		t.Errorf("csrf token %q is not valid for the login action", result.Token)
	}
}

func TestImageReferenceValidationEndpoint(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantValid  bool
		wantStatus int
	}{
		{name: "valid image", body: `{"reference":"nginx:1.27"}`, wantValid: true, wantStatus: http.StatusOK},
		{name: "invalid image", body: `{"reference":"not a valid image ref"}`, wantValid: false, wantStatus: http.StatusOK},
		{name: "malformed request", body: `{`, wantValid: false, wantStatus: http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := serveAPIRequest(t, http.MethodPost, "/api/v1/appdeployment/validate/imagereference", tc.body)
			if response.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", response.Code, tc.wantStatus, response.Body.String())
			}
			if tc.wantStatus != http.StatusOK {
				return
			}

			var result validation.ImageReferenceValidity
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatalf("decode image validation response: %v", err)
			}
			if result.Valid != tc.wantValid {
				t.Errorf("image validity = %t, want %t (reason: %q)", result.Valid, tc.wantValid, result.Reason)
			}
		})
	}
}

func TestProtocolValidationEndpoint(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantValid  bool
		wantStatus int
	}{
		{name: "UDP external service is invalid", body: `{"protocol":"UDP","isExternal":true}`, wantValid: false, wantStatus: http.StatusOK},
		{name: "UDP internal service is valid", body: `{"protocol":"UDP","isExternal":false}`, wantValid: true, wantStatus: http.StatusOK},
		{name: "TCP external service is valid", body: `{"protocol":"TCP","isExternal":true}`, wantValid: true, wantStatus: http.StatusOK},
		{name: "malformed request", body: `{`, wantStatus: http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := serveAPIRequest(t, http.MethodPost, "/api/v1/appdeployment/validate/protocol", tc.body)
			if response.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", response.Code, tc.wantStatus, response.Body.String())
			}
			if tc.wantStatus != http.StatusOK {
				return
			}

			var result validation.ProtocolValidity
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatalf("decode protocol validation response: %v", err)
			}
			if result.Valid != tc.wantValid {
				t.Errorf("protocol validity = %t, want %t", result.Valid, tc.wantValid)
			}
		})
	}
}

func TestAvailableProtocolsEndpoint(t *testing.T) {
	response := serveAPIRequest(t, http.MethodGet, "/api/v1/appdeployment/protocols", "")
	if response.Code != http.StatusOK {
		t.Fatalf("GET protocols status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}

	var result struct {
		Protocols []corev1.Protocol `json:"protocols"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode protocols response: %v", err)
	}
	if len(result.Protocols) != 2 || result.Protocols[0] != corev1.ProtocolTCP || result.Protocols[1] != corev1.ProtocolUDP {
		t.Errorf("available protocols = %v, want [TCP UDP]", result.Protocols)
	}
}

func TestResourceGETHandlersReportMissingKubernetesConfiguration(t *testing.T) {
	container, err := CreateHTTPAPIHandler(nil)
	if err != nil {
		t.Fatalf("create API handler: %v", err)
	}
	paths := []string{
		"/api/v1/pod/default",
		"/api/v1/pod/default/web",
		"/api/v1/replicationcontroller/default",
		"/api/v1/replicaset/default",
		"/api/v1/deployment/default",
		"/api/v1/daemonset/default",
		"/api/v1/statefulset/default",
		"/api/v1/job/default",
		"/api/v1/cronjob/default",
		"/api/v1/horizontalpodautoscaler/default",
		"/api/v1/namespace",
		"/api/v1/event/default",
		"/api/v1/secret/default",
		"/api/v1/configmap/default",
		"/api/v1/service/default",
		"/api/v1/serviceaccount/default",
		"/api/v1/ingress/default",
		"/api/v1/networkpolicy/default",
		"/api/v1/node",
		"/api/v1/clusterrole",
		"/api/v1/clusterrolebinding",
		"/api/v1/role/default",
		"/api/v1/rolebinding/default",
		"/api/v1/persistentvolume",
		"/api/v1/storageclass",
		"/api/v1/ingressclass",
		"/api/v1/crd",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			response := httptest.NewRecorder()
			container.ServeHTTP(response, request)
			if response.Code != http.StatusInternalServerError {
				t.Errorf("GET %s status = %d, want %d: %s", path, response.Code, http.StatusInternalServerError, response.Body.String())
			}
		})
	}
}

func TestResourceDetailGETHandlersReportMissingKubernetesConfiguration(t *testing.T) {
	container, err := CreateHTTPAPIHandler(nil)
	if err != nil {
		t.Fatalf("create API handler: %v", err)
	}
	paths := []string{
		"/api/v1/replicationcontroller/default/controller",
		"/api/v1/replicationcontroller/default/controller/pod",
		"/api/v1/replicationcontroller/default/controller/event",
		"/api/v1/replicationcontroller/default/controller/service",
		"/api/v1/replicaset/default/replica",
		"/api/v1/replicaset/default/replica/pod",
		"/api/v1/replicaset/default/replica/service",
		"/api/v1/replicaset/default/replica/event",
		"/api/v1/pod/default/pod",
		"/api/v1/pod/default/pod/container",
		"/api/v1/pod/default/pod/event",
		"/api/v1/pod/default/pod/persistentvolumeclaim",
		"/api/v1/deployment/default/deployment",
		"/api/v1/deployment/default/deployment/event",
		"/api/v1/deployment/default/deployment/oldreplicaset",
		"/api/v1/deployment/default/deployment/newreplicaset",
		"/api/v1/daemonset/default/daemonset",
		"/api/v1/daemonset/default/daemonset/pod",
		"/api/v1/daemonset/default/daemonset/service",
		"/api/v1/daemonset/default/daemonset/event",
		"/api/v1/deployment/default/deployment/horizontalpodautoscaler",
		"/api/v1/horizontalpodautoscaler/default/hpa",
		"/api/v1/job/default/job",
		"/api/v1/job/default/job/pod",
		"/api/v1/job/default/job/event",
		"/api/v1/cronjob/default/cronjob",
		"/api/v1/cronjob/default/cronjob/job",
		"/api/v1/cronjob/default/cronjob/event",
		"/api/v1/namespace/default",
		"/api/v1/namespace/default/event",
		"/api/v1/secret/default/secret",
		"/api/v1/configmap/default/configmap",
		"/api/v1/service/default/service",
		"/api/v1/service/default/service/event",
		"/api/v1/service/default/service/pod",
		"/api/v1/service/default/service/ingress",
		"/api/v1/serviceaccount/default/account",
		"/api/v1/serviceaccount/default/account/secret",
		"/api/v1/serviceaccount/default/account/imagepullsecret",
		"/api/v1/ingress/default/ingress",
		"/api/v1/ingress/default/ingress/event",
		"/api/v1/networkpolicy/default/policy",
		"/api/v1/statefulset/default/statefulset",
		"/api/v1/statefulset/default/statefulset/pod",
		"/api/v1/statefulset/default/statefulset/event",
		"/api/v1/node/node-a",
		"/api/v1/node/node-a/event",
		"/api/v1/node/node-a/pod",
		"/api/v1/_raw/pod/namespace/default/name/pod-a",
		"/api/v1/_raw/node/name/node-a",
		"/api/v1/scale/deployment/default/deployment",
		"/api/v1/scale/node/node-a",
		"/api/v1/clusterrole/admin",
		"/api/v1/clusterrolebinding/admin-binding",
		"/api/v1/role/default/reader",
		"/api/v1/rolebinding/default/reader-binding",
		"/api/v1/persistentvolume/pv-a",
		"/api/v1/persistentvolume/namespace/default/name/pv-a",
		"/api/v1/persistentvolumeclaim/default/pvc-a",
		"/api/v1/poddisruptionbudget/default/pdb",
		"/api/v1/crd/widgets.example.test",
		"/api/v1/crd/default/widgets.example.test/object",
		"/api/v1/crd/default/widgets.example.test/widget-a",
		"/api/v1/crd/default/widgets.example.test/widget-a/event",
		"/api/v1/storageclass/standard",
		"/api/v1/storageclass/standard/persistentvolume",
		"/api/v1/ingressclass/nginx",
		"/api/v1/log/source/default/deployment/web",
		"/api/v1/log/default/pod-a",
		"/api/v1/log/default/pod-a/container-a",
		"/api/v1/log/file/default/pod-a/container-a",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			response := httptest.NewRecorder()
			container.ServeHTTP(response, request)
			if response.Code != http.StatusInternalServerError && response.Code != http.StatusUnauthorized {
				t.Errorf("GET %s status = %d, want %d or %d: %s", path, response.Code, http.StatusInternalServerError, http.StatusUnauthorized, response.Body.String())
			}
		})
	}
}

func serveAPIRequest(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	container, err := CreateHTTPAPIHandler(nil)
	if err != nil {
		t.Fatalf("create API handler: %v", err)
	}

	request, err := http.NewRequest(method, path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if body != "" {
		request.Header.Set("Content-Type", restful.MIME_JSON)
	}
	response := httptest.NewRecorder()
	container.ServeHTTP(response, request)
	return response
}

func TestShouldDoCsrfValidation(t *testing.T) {
	cases := []struct {
		request  *restful.Request
		expected bool
	}{
		{
			&restful.Request{
				Request: &http.Request{
					Method: "PUT",
				},
			},
			false,
		},
		{
			&restful.Request{
				Request: &http.Request{
					Method: "POST",
				},
			},
			true,
		},
	}
	for _, c := range cases {
		actual := shouldDoCsrfValidation(c.request)
		if actual != c.expected {
			t.Errorf("shouldDoCsrfValidation(%#v) returns %#v, expected %#v", c.request, actual, c.expected)
		}
	}
}

func TestFormatRequestLog(t *testing.T) {
	cases := []struct {
		method      string
		uri         string
		content     map[string]string
		expected    string
		apiLogLevel klog.Level
	}{
		{
			"PUT",
			"/api/v1/pod",
			map[string]string{},
			"Incoming HTTP/1.1 PUT /api/v1/pod request",
			args.LogLevelDefault,
		},
		{
			"PUT",
			"/api/v1/pod",
			map[string]string{},
			"",
			args.LogLevelMinimal,
		},
		{
			"POST",
			"/api/v1/login",
			map[string]string{"password": "abc123"},
			"Incoming HTTP/1.1 POST /api/v1/login request from { content hidden }: { content hidden }",
			args.LogLevelDefault,
		},
		{
			"POST",
			"/api/v1/login",
			map[string]string{},
			"",
			args.LogLevelMinimal,
		},
		{
			"POST",
			"/api/v1/login",
			map[string]string{"password": "abc123"},
			"Incoming HTTP/1.1 POST /api/v1/login request from : {\"password\":\"abc123\"}",
			args.LogLevelDebug,
		},
	}

	for _, c := range cases {
		jsonValue, _ := json.Marshal(c.content)

		req, err := http.NewRequest(c.method, c.uri, bytes.NewReader(jsonValue))
		req.Header.Set("Content-Type", "application/json")

		if err != nil {
			t.Error("Cannot mockup request")
		}

		_ = pflag.Set("v", c.apiLogLevel.String())

		var restfulRequest restful.Request
		restfulRequest.Request = req

		actual := formatRequestLog(&restfulRequest)
		if !strings.Contains(actual, c.expected) {
			t.Errorf("formatRequestLog(%#v) returns %#v, expected to contain %#v", req, actual, c.expected)
		}
	}
}
