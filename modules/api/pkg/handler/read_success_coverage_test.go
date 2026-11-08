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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"k8s.io/dashboard/api/pkg/handler"
	"k8s.io/dashboard/api/pkg/integration"
)

func TestResourceDetailRawResourceAndLogReadHandlersSucceed(t *testing.T) {
	const (
		deployment = `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"web","namespace":"default"},"spec":{"replicas":1,"selector":{"matchLabels":{"app":"web"}},"template":{"metadata":{"labels":{"app":"web"}},"spec":{"containers":[{"name":"main","image":"example/web"}]}}}}`
		replicaSet = `{"apiVersion":"apps/v1","kind":"ReplicaSet","metadata":{"name":"web","namespace":"default","uid":"rs-uid"},"spec":{"replicas":1,"selector":{"matchLabels":{"app":"web"}}}}`
		pod        = `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"web","namespace":"default","uid":"pod-uid","ownerReferences":[{"apiVersion":"apps/v1","kind":"ReplicaSet","name":"web","uid":"rs-uid","controller":true}]},"spec":{"containers":[{"name":"main","image":"example/web"}]},"status":{"phase":"Running"}}`
	)
	initializeAPITestClientWithTransport(t, func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/apis/apps/v1/namespaces/default/deployments/web":
			return apiResponse(request, http.StatusOK, deployment), nil
		case "/apis/apps/v1/namespaces/default/replicasets/web":
			return apiResponse(request, http.StatusOK, replicaSet), nil
		case "/apis/apps/v1/namespaces/default/replicasets":
			return apiResponse(request, http.StatusOK, `{"apiVersion":"apps/v1","kind":"ReplicaSetList","items":[]}`), nil
		case "/api/v1/namespaces/default/pods":
			return apiResponse(request, http.StatusOK, `{"apiVersion":"v1","kind":"PodList","items":[`+pod+`]}`), nil
		case "/api/v1/namespaces/default/events":
			return apiResponse(request, http.StatusOK, `{"apiVersion":"v1","kind":"EventList","items":[]}`), nil
		case "/api/v1/namespaces/default/pods/web":
			return apiResponse(request, http.StatusOK, pod), nil
		case "/api/v1/namespaces/default/pods/web/log":
			return apiResponse(request, http.StatusOK, "2026-11-07T00:00:00Z container started\n"), nil
		case "/api":
			return apiResponse(request, http.StatusOK, `{"kind":"APIVersions","versions":["v1"]}`), nil
		case "/apis":
			return apiResponse(request, http.StatusOK, `{"kind":"APIGroupList","groups":[]}`), nil
		case "/api/v1":
			return apiResponse(request, http.StatusOK, `{"kind":"APIResourceList","groupVersion":"v1","resources":[{"name":"pods","singularName":"pod","namespaced":true,"kind":"Pod","verbs":["get"]}]}`), nil
		default:
			t.Errorf("unexpected Kubernetes request: %s %s", request.Method, request.URL.String())
			return apiResponse(request, http.StatusNotFound, `{"kind":"Status","status":"Failure","message":"unexpected Kubernetes request","code":404}`), nil
		}
	})

	api, err := handler.CreateHTTPAPIHandler(integration.NewIntegrationManager())
	if err != nil {
		t.Fatalf("create API handler: %v", err)
	}
	for _, path := range []string{
		"/api/v1/deployment/default/web",
		"/api/v1/replicaset/default/web/pod",
		"/api/v1/_raw/pod/namespace/default/name/web",
		"/api/v1/log/source/default/web/pod",
		"/api/v1/log/default/web/main?offsetFrom=0&offsetTo=10",
		"/api/v1/log/file/default/web/main",
		"/api/v1/namespace/default/event",
	} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			request.Header.Set("Authorization", "Bearer read-success-test-token")
			response := httptest.NewRecorder()
			api.ServeHTTP(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("GET %s = %d, want %d: %s", path, response.Code, http.StatusOK, response.Body.String())
			}
			if strings.Contains(path, "/log/file/") && !strings.Contains(response.Body.String(), "container started") {
				t.Errorf("log file response = %q; want streamed log contents", response.Body.String())
			}
		})
	}
}
