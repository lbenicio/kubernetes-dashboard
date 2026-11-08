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

package deployment

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"k8s.io/client-go/rest"
)

type deploymentRoundTripper func(*http.Request) (*http.Response, error)

func (f deploymentRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestDeployAppFromFileCreatesNamespacedAndClusterResources(t *testing.T) {
	for _, test := range []struct {
		name           string
		kind           string
		namespaced     bool
		namespace      string
		manifest       string
		wantCreatePath string
	}{
		{
			name:           "namespace supplied by request",
			kind:           "Pod",
			namespaced:     true,
			namespace:      "team-a",
			manifest:       "apiVersion: v1\nkind: Pod\nmetadata:\n  name: web\n",
			wantCreatePath: "/api/v1/namespaces/team-a/pods",
		},
		{
			name:           "all namespaces uses manifest namespace",
			kind:           "Pod",
			namespaced:     true,
			namespace:      "_all",
			manifest:       "apiVersion: v1\nkind: Pod\nmetadata:\n  name: web\n  namespace: manifest-ns\n",
			wantCreatePath: "/api/v1/namespaces/manifest-ns/pods",
		},
		{
			name:           "cluster scoped resource",
			kind:           "Node",
			namespaced:     false,
			namespace:      "ignored",
			manifest:       "apiVersion: v1\nkind: Node\nmetadata:\n  name: worker\n",
			wantCreatePath: "/api/v1/nodes",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var createPath string
			config := &rest.Config{
				Host: "http://kubernetes.test",
				Transport: deploymentRoundTripper(func(request *http.Request) (*http.Response, error) {
					var statusCode int
					var body string
					if request.Method == http.MethodGet && request.URL.Path == "/api/v1" {
						statusCode = http.StatusOK
						body = `{"kind":"APIResourceList","apiVersion":"v1","resources":[{"name":"pods","singularName":"pod","namespaced":true,"kind":"Pod"},{"name":"nodes","singularName":"node","namespaced":false,"kind":"Node"}]}`
					} else if request.Method == http.MethodPost {
						createPath = request.URL.Path
						statusCode = http.StatusCreated
						body = `{"apiVersion":"v1","kind":"` + test.kind + `","metadata":{"name":"created"}}`
					} else {
						statusCode = http.StatusNotFound
						body = `{"kind":"Status","apiVersion":"v1","status":"Failure","code":404}`
					}
					return &http.Response{StatusCode: statusCode, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
				}),
			}
			deployed, err := DeployAppFromFile(config, &AppDeploymentFromFileSpec{Namespace: test.namespace, Content: test.manifest})
			if err != nil || !deployed {
				t.Fatalf("DeployAppFromFile() = %v, %v", deployed, err)
			}
			if createPath != test.wantCreatePath {
				t.Errorf("dynamic create path = %q, want %q", createPath, test.wantCreatePath)
			}
		})
	}
}

func TestDeployAppFromFileRejectsUnknownResourceKind(t *testing.T) {
	config := &rest.Config{
		Host: "http://kubernetes.test",
		Transport: deploymentRoundTripper(func(request *http.Request) (*http.Response, error) {
			body := `{"kind":"APIResourceList","apiVersion":"v1","resources":[{"name":"pods","singularName":"pod","namespaced":true,"kind":"Pod"}]}`
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
		}),
	}
	deployed, err := DeployAppFromFile(config, &AppDeploymentFromFileSpec{Namespace: "default", Content: "apiVersion: v1\nkind: Unknown\nmetadata:\n  name: object\n"})
	if deployed || err == nil || !strings.Contains(err.Error(), "unknown resource kind: Unknown") {
		t.Errorf("DeployAppFromFile(unknown kind) = %v, %v", deployed, err)
	}
}

func TestDeployAppFromFileReportsMalformedDocumentsDiscoveryAndCreateErrors(t *testing.T) {
	t.Run("malformed document", func(t *testing.T) {
		deployed, err := DeployAppFromFile(&rest.Config{}, &AppDeploymentFromFileSpec{Content: "apiVersion: [\nkind: Pod"})
		if deployed || err == nil {
			t.Fatalf("DeployAppFromFile(malformed) = %v, %v; want a decode error", deployed, err)
		}
	})

	t.Run("malformed group version fails discovery", func(t *testing.T) {
		config := &rest.Config{
			Host: "http://kubernetes.test",
			Transport: deploymentRoundTripper(func(request *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"kind":"Status","apiVersion":"v1","status":"Failure","code":404}`)),
					Request:    request,
				}, nil
			}),
		}
		deployed, err := DeployAppFromFile(config, &AppDeploymentFromFileSpec{Content: "apiVersion: invalid/group/version\nkind: Pod\nmetadata:\n  name: web\n"})
		if deployed || err == nil {
			t.Fatalf("DeployAppFromFile(invalid version) = %v, %v; want discovery error", deployed, err)
		}
	})

	t.Run("invalid client config fails before discovery", func(t *testing.T) {
		deployed, err := DeployAppFromFile(&rest.Config{Host: "%"}, &AppDeploymentFromFileSpec{
			Namespace: "default",
			Content:   "apiVersion: v1\nkind: Pod\nmetadata:\n  name: web\n",
		})
		if deployed || err == nil {
			t.Fatalf("DeployAppFromFile(invalid config) = %v, %v; want a REST client error", deployed, err)
		}
	})

	t.Run("resource creation failure", func(t *testing.T) {
		config := &rest.Config{
			Host: "http://kubernetes.test",
			Transport: deploymentRoundTripper(func(request *http.Request) (*http.Response, error) {
				statusCode := http.StatusOK
				body := `{"kind":"APIResourceList","apiVersion":"v1","resources":[{"name":"pods","namespaced":true,"kind":"Pod"}]}`
				if request.Method == http.MethodPost {
					statusCode = http.StatusForbidden
					body = `{"kind":"Status","apiVersion":"v1","status":"Failure","message":"create denied","reason":"Forbidden","code":403}`
				}
				return &http.Response{StatusCode: statusCode, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
			}),
		}
		deployed, err := DeployAppFromFile(config, &AppDeploymentFromFileSpec{Namespace: "default", Content: "apiVersion: v1\nkind: Pod\nmetadata:\n  name: web\n"})
		if deployed || err == nil || !strings.Contains(err.Error(), "create denied") {
			t.Fatalf("DeployAppFromFile(create failure) = %v, %v; want a localized forbidden error", deployed, err)
		}
	})
}
