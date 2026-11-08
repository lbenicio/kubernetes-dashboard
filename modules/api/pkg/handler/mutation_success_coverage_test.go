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

package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/emicklei/go-restful/v3"
	"golang.org/x/net/xsrftoken"

	"k8s.io/dashboard/api/pkg/handler"
	"k8s.io/dashboard/api/pkg/integration"
	"k8s.io/dashboard/csrf"
)

func TestMutationHandlersCompleteSuccessfulOperations(t *testing.T) {
	deployment := `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"web","namespace":"default","uid":"deployment-uid","annotations":{"deployment.kubernetes.io/revision":"2"}},"spec":{"selector":{"matchLabels":{"app":"web"}},"template":{"metadata":{"labels":{"app":"web"}},"spec":{"containers":[{"name":"web","image":"nginx"}]}}}}`
	daemonSet := `{"apiVersion":"apps/v1","kind":"DaemonSet","metadata":{"name":"agents","namespace":"default"},"spec":{"selector":{"matchLabels":{"app":"agents"}},"template":{"metadata":{"labels":{"app":"agents"}},"spec":{"containers":[{"name":"agent","image":"example/agent"}]}}}}`
	statefulSet := `{"apiVersion":"apps/v1","kind":"StatefulSet","metadata":{"name":"database","namespace":"default"},"spec":{"selector":{"matchLabels":{"app":"database"}},"template":{"metadata":{"labels":{"app":"database"}},"spec":{"containers":[{"name":"database","image":"example/database"}]}}}}`
	cronJob := `{"apiVersion":"batch/v1","kind":"CronJob","metadata":{"name":"backup","namespace":"default","uid":"cron-uid"},"spec":{"jobTemplate":{"metadata":{"labels":{"app":"backup"}},"spec":{"template":{"spec":{"containers":[{"name":"backup","image":"example/backup"}],"restartPolicy":"Never"}}}}}}`
	replicationController := `{"apiVersion":"v1","kind":"ReplicationController","metadata":{"name":"web","namespace":"default"},"spec":{"replicas":1,"selector":{"app":"web"},"template":{"metadata":{"labels":{"app":"web"}},"spec":{"containers":[{"name":"web","image":"nginx"}]}}}}`
	replicaSets := `{"apiVersion":"apps/v1","kind":"ReplicaSetList","items":[{"metadata":{"name":"web-old","namespace":"default","uid":"rs-uid","labels":{"app":"web"},"annotations":{"deployment.kubernetes.io/revision":"1"},"ownerReferences":[{"apiVersion":"apps/v1","kind":"Deployment","name":"web","uid":"deployment-uid","controller":true}]},"spec":{"template":{"spec":{"containers":[{"name":"web","image":"nginx:old"}]}}}}]}`
	scale := `{"apiVersion":"autoscaling/v1","kind":"Scale","metadata":{"name":"web","namespace":"default"},"spec":{"replicas":3},"status":{"replicas":2}}`
	updatedScale := `{"apiVersion":"autoscaling/v1","kind":"Scale","metadata":{"name":"web","namespace":"default"},"spec":{"replicas":4},"status":{"replicas":2}}`
	pausedDeployment := strings.Replace(deployment, `"selector":`, `"paused":true,"selector":`, 1)
	deploymentReads := 0

	initializeAPITestClientWithTransport(t, func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/apis/apps/v1/namespaces/default/deployments/web":
			if request.Method == http.MethodGet {
				deploymentReads++
				if deploymentReads == 5 {
					return apiResponse(request, http.StatusOK, pausedDeployment), nil
				}
			}
			return apiResponse(request, http.StatusOK, deployment), nil
		case "/apis/apps/v1/namespaces/default/daemonsets/agents":
			return apiResponse(request, http.StatusOK, daemonSet), nil
		case "/apis/apps/v1/namespaces/default/statefulsets/database":
			return apiResponse(request, http.StatusOK, statefulSet), nil
		case "/apis/batch/v1/namespaces/default/cronjobs/backup":
			return apiResponse(request, http.StatusOK, cronJob), nil
		case "/apis/batch/v1/namespaces/default/jobs":
			return apiResponse(request, http.StatusCreated, `{"apiVersion":"batch/v1","kind":"Job","metadata":{"name":"backup-manual-abc","namespace":"default"}}`), nil
		case "/apis":
			return apiResponse(request, http.StatusOK, `{"kind":"APIGroupList","groups":[{"name":"apps","versions":[{"groupVersion":"apps/v1","version":"v1"}],"preferredVersion":{"groupVersion":"apps/v1","version":"v1"}}]}`), nil
		case "/apis/apps/v1":
			return apiResponse(request, http.StatusOK, `{"kind":"APIResourceList","groupVersion":"apps/v1","resources":[{"name":"deployments","singularName":"deployment","namespaced":true,"kind":"Deployment","verbs":["get","list"]},{"name":"deployments/scale","singularName":"","namespaced":true,"kind":"Scale","group":"autoscaling","version":"v1","verbs":["get","update","patch"]}]}`), nil
		case "/apis/apps/v1/namespaces/default/replicasets":
			return apiResponse(request, http.StatusOK, replicaSets), nil
		case "/apis/apps/v1/namespaces/default/deployments/web/scale":
			if request.Method == http.MethodPut {
				return apiResponse(request, http.StatusOK, updatedScale), nil
			}
			return apiResponse(request, http.StatusOK, scale), nil
		case "/api/v1/namespaces/default/replicationcontrollers/web":
			return apiResponse(request, http.StatusOK, replicationController), nil
		case "/api/v1/namespaces":
			return apiResponse(request, http.StatusCreated, `{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"team-a"}}`), nil
		case "/api/v1/namespaces/default/secrets":
			return apiResponse(request, http.StatusCreated, `{"apiVersion":"v1","kind":"Secret","metadata":{"name":"registry","namespace":"default"},"type":"kubernetes.io/dockercfg"}`), nil
		default:
			return apiResponse(request, http.StatusNotFound, `{"apiVersion":"v1","kind":"Status","status":"Failure","reason":"NotFound","code":404}`), nil
		}
	})

	container, err := handler.CreateHTTPAPIHandler(integration.NewIntegrationManager())
	if err != nil {
		t.Fatalf("create API handler: %v", err)
	}
	for _, test := range []struct {
		name       string
		method     string
		path       string
		body       string
		action     string
		wantStatus int
	}{
		{name: "pause deployment", method: http.MethodPut, path: "/api/v1/deployment/default/web/pause", wantStatus: http.StatusOK},
		{name: "rollback deployment", method: http.MethodPut, path: "/api/v1/deployment/default/web/rollback", body: `{"revision":"1"}`, wantStatus: http.StatusOK},
		{name: "restart deployment", method: http.MethodPut, path: "/api/v1/deployment/default/web/restart", wantStatus: http.StatusOK},
		{name: "resume deployment", method: http.MethodPut, path: "/api/v1/deployment/default/web/resume", wantStatus: http.StatusOK},
		{name: "restart daemonset", method: http.MethodPut, path: "/api/v1/daemonset/default/agents/restart", wantStatus: http.StatusOK},
		{name: "restart statefulset", method: http.MethodPut, path: "/api/v1/statefulset/default/database/restart", wantStatus: http.StatusOK},
		{name: "trigger cronjob", method: http.MethodPut, path: "/api/v1/cronjob/default/backup/trigger", wantStatus: http.StatusOK},
		{name: "scale deployment", method: http.MethodPut, path: "/api/v1/scale/deployment/default/web?scaleBy=4", wantStatus: http.StatusOK},
		{name: "read deployment replicas", method: http.MethodGet, path: "/api/v1/scale/deployment/default/web", wantStatus: http.StatusOK},
		{name: "update replication controller replicas", method: http.MethodPost, path: "/api/v1/replicationcontroller/default/web/update/pod", body: `{"replicas":4}`, action: "replicationcontroller", wantStatus: http.StatusAccepted},
		{name: "create namespace", method: http.MethodPost, path: "/api/v1/namespace", body: `{"name":"team-a"}`, action: "namespace", wantStatus: http.StatusCreated},
		{name: "create image pull secret", method: http.MethodPost, path: "/api/v1/secret", body: `{"name":"registry","namespace":"default","data":"e30="}`, action: "secret", wantStatus: http.StatusCreated},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			request.Header.Set("Authorization", "Bearer mutation-success-test-token")
			request.Header.Set("Content-Type", restful.MIME_JSON)
			if test.action != "" {
				request.Header.Set("X-CSRF-TOKEN", xsrftoken.Generate(csrf.Key(), "none", test.action))
			}
			response := httptest.NewRecorder()
			container.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("%s returned %d, want %d: %s", test.name, response.Code, test.wantStatus, response.Body.String())
			}
		})
	}
}

func TestRawResourceMutationHandlersCompleteSuccessfulOperations(t *testing.T) {
	storedPod := `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"web","namespace":"default","resourceVersion":"7","labels":{"app":"web"}},"spec":{"containers":[{"name":"web","image":"nginx"}]}}`
	initializeAPITestClientWithTransport(t, func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api":
			return apiResponse(request, http.StatusOK, `{"kind":"APIVersions","versions":["v1"]}`), nil
		case "/apis":
			return apiResponse(request, http.StatusOK, `{"kind":"APIGroupList","groups":[]}`), nil
		case "/api/v1":
			return apiResponse(request, http.StatusOK, `{"kind":"APIResourceList","groupVersion":"v1","resources":[{"name":"pods","singularName":"pod","namespaced":true,"kind":"Pod","verbs":["get","patch","delete"]}]}`), nil
		case "/api/v1/namespaces/default/pods/web":
			switch request.Method {
			case http.MethodGet:
				return apiResponse(request, http.StatusOK, storedPod), nil
			case http.MethodPatch:
				return apiResponse(request, http.StatusOK, storedPod), nil
			case http.MethodDelete:
				return apiResponse(request, http.StatusOK, `{"kind":"Status","status":"Success","code":200}`), nil
			}
		}
		return apiResponse(request, http.StatusNotFound, `{"kind":"Status","status":"Failure","reason":"NotFound","code":404}`), nil
	})
	container, err := handler.CreateHTTPAPIHandler(integration.NewIntegrationManager())
	if err != nil {
		t.Fatalf("create API handler: %v", err)
	}

	for _, test := range []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{
			name:       "update resource",
			method:     http.MethodPut,
			path:       "/api/v1/_raw/pod/namespace/default/name/web",
			body:       `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"web","namespace":"default","labels":{"app":"web","tier":"frontend"}},"spec":{"containers":[{"name":"web","image":"nginx"}]}}`,
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "delete resource",
			method:     http.MethodDelete,
			path:       "/api/v1/_raw/pod/namespace/default/name/web?propagation=Background&deleteNow=true",
			wantStatus: http.StatusNoContent,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			request.Header.Set("Authorization", "Bearer raw-mutation-test-token")
			request.Header.Set("Content-Type", restful.MIME_JSON)
			response := httptest.NewRecorder()
			container.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("%s returned %d, want %d: %s", test.name, response.Code, test.wantStatus, response.Body.String())
			}
		})
	}
}

func TestNodeDrainHandlerAcceptsSuccessfulCordonAndEmptyDrain(t *testing.T) {
	const node = `{"apiVersion":"v1","kind":"Node","metadata":{"name":"worker-1"},"spec":{"unschedulable":true}}`
	initializeAPITestClientWithTransport(t, func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/v1/nodes/worker-1":
			if request.Method != http.MethodGet && request.Method != http.MethodPatch {
				return apiResponse(request, http.StatusMethodNotAllowed, `{"kind":"Status","status":"Failure","code":405}`), nil
			}
			return apiResponse(request, http.StatusOK, node), nil
		case "/api/v1/pods":
			return apiResponse(request, http.StatusOK, `{"apiVersion":"v1","kind":"PodList","items":[]}`), nil
		default:
			t.Errorf("unexpected Kubernetes request during drain: %s %s", request.Method, request.URL.String())
			return apiResponse(request, http.StatusNotFound, `{"kind":"Status","status":"Failure","code":404}`), nil
		}
	})

	api, err := handler.CreateHTTPAPIHandler(integration.NewIntegrationManager())
	if err != nil {
		t.Fatalf("create API handler: %v", err)
	}
	request := httptest.NewRequest(http.MethodPut, "/api/v1/node/worker-1/drain", strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Bearer drain-success-test-token")
	request.Header.Set("Content-Type", restful.MIME_JSON)
	response := httptest.NewRecorder()
	api.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("node drain status = %d; want %d: %s", response.Code, http.StatusAccepted, response.Body.String())
	}
}
