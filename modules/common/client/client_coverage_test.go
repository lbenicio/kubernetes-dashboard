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
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakekubernetes "k8s.io/client-go/kubernetes/fake"
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

	if err := cacheFlag.Value.Set("true"); err != nil {
		t.Fatal(err)
	}
	if got, err := Client(request); err != nil || got == nil {
		t.Fatalf("cached Client() = %v, %v", got, err)
	}
	if got, err := APIExtensionsClient(request); err != nil || got == nil {
		t.Fatalf("cached APIExtensionsClient() = %v, %v", got, err)
	}
	if err := cacheFlag.Value.Set("false"); err != nil {
		t.Fatal(err)
	}

	fakeInClusterClient := fakekubernetes.NewSimpleClientset()
	inClusterClient = fakeInClusterClient
	if got := InClusterClient(); got != fakeInClusterClient {
		t.Fatalf("InClusterClient() = %v, want cached in-cluster client", got)
	}
	inClusterClient = nil

	if CanI(httptest.NewRequest("GET", "https://dashboard.example.test", nil), nil) {
		t.Fatal("CanI() granted a request without an authorization header")
	}
	if got := setConfigRateLimitDefaults(nil); got != nil {
		t.Fatalf("setConfigRateLimitDefaults(nil) = %v", got)
	}
}

func TestCanIReportsAllowedDeniedAndKubernetesErrors(t *testing.T) {
	cacheFlag := pflag.Lookup("cache-enabled")
	previousCache := cacheFlag.Value.String()
	if err := cacheFlag.Value.Set("false"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cacheFlag.Value.Set(previousCache) })

	anonymousFlag := pflag.Lookup("anonymous-admin")
	previousAnonymous := anonymousFlag.Value.String()
	if err := anonymousFlag.Value.Set("false"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = anonymousFlag.Value.Set(previousAnonymous) })

	previousConfig, previousClient := baseConfig, inClusterClient
	t.Cleanup(func() {
		baseConfig = previousConfig
		inClusterClient = previousClient
	})

	for _, test := range []struct {
		name       string
		statusCode int
		allowed    bool
		want       bool
	}{
		{name: "allowed", statusCode: http.StatusCreated, allowed: true, want: true},
		{name: "denied", statusCode: http.StatusCreated, allowed: false, want: false},
		{name: "Kubernetes API error", statusCode: http.StatusInternalServerError, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			baseConfig = &rest.Config{
				Host: "https://kubernetes.example.test",
				WrapTransport: func(http.RoundTripper) http.RoundTripper {
					return roundTripFunc(func(r *http.Request) (*http.Response, error) {
						if r.Method != http.MethodPost || r.URL.Path != "/apis/authorization.k8s.io/v1/selfsubjectaccessreviews" {
							t.Errorf("request = %s %s", r.Method, r.URL.Path)
						}
						if got := r.Header.Get("Authorization"); got != "Bearer allowed-token" {
							t.Errorf("Authorization = %q, want bearer token", got)
						}
						body := `{"kind":"Status","status":"Failure","reason":"InternalError","code":500}`
						if test.statusCode == http.StatusCreated {
							payload, err := json.Marshal(authorizationv1.SelfSubjectAccessReview{
								Status: authorizationv1.SubjectAccessReviewStatus{Allowed: test.allowed},
							})
							if err != nil {
								return nil, err
							}
							body = string(payload)
						}
						return &http.Response{
							StatusCode: test.statusCode,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    r,
						}, nil
					})
				},
			}
			inClusterClient = nil
			request := httptest.NewRequest(http.MethodGet, "https://dashboard.example.test", nil)
			SetAuthorizationHeader(request, "allowed-token")
			review := &authorizationv1.SelfSubjectAccessReview{
				Spec: authorizationv1.SelfSubjectAccessReviewSpec{
					ResourceAttributes: &authorizationv1.ResourceAttributes{Resource: "pods", Verb: "list"},
				},
			}

			if got := CanI(request, review); got != test.want {
				t.Fatalf("CanI() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestClientFactoriesRequireInitialization(t *testing.T) {
	previousConfig, previousClient := baseConfig, inClusterClient
	baseConfig = nil
	inClusterClient = nil
	t.Cleanup(func() {
		baseConfig = previousConfig
		inClusterClient = previousClient
	})
	request := httptest.NewRequest(http.MethodGet, "https://dashboard.example.test", nil)
	SetAuthorizationHeader(request, "test-token")
	if _, err := Client(request); err == nil {
		t.Fatal("Client() succeeded before initialization")
	}
	if _, err := APIExtensionsClient(request); err == nil {
		t.Fatal("APIExtensionsClient() succeeded before initialization")
	}
	if _, err := Config(request); err == nil {
		t.Fatal("Config() succeeded before initialization")
	}
	if InClusterClient() != nil {
		t.Fatal("InClusterClient() returned a client before initialization")
	}
}

func TestClientFactoriesReturnConfigurationErrors(t *testing.T) {
	previousConfig, previousClient := baseConfig, inClusterClient
	baseConfig = &rest.Config{Host: "%"}
	inClusterClient = nil
	t.Cleanup(func() {
		baseConfig = previousConfig
		inClusterClient = previousClient
	})
	request := httptest.NewRequest(http.MethodGet, "https://dashboard.example.test", nil)
	SetAuthorizationHeader(request, "test-token")
	if _, err := Client(request); err == nil {
		t.Fatal("Client() accepted malformed Kubernetes configuration")
	}
	if _, err := APIExtensionsClient(request); err == nil {
		t.Fatal("APIExtensionsClient() accepted malformed Kubernetes configuration")
	}
	if _, err := RestClientForHost("%"); err == nil {
		t.Fatal("RestClientForHost() accepted a malformed host")
	}
	if _, err := APIExtensionsClient(httptest.NewRequest(http.MethodGet, "https://dashboard.example.test", nil)); err == nil {
		t.Fatal("APIExtensionsClient() accepted a request without authorization")
	}
}

func TestCachedClientFactoriesUseRequestGetterForNoCacheLists(t *testing.T) {
	previousConfig, previousClient := baseConfig, inClusterClient
	baseConfig = &rest.Config{
		Host: "https://kubernetes.example.test",
		WrapTransport: func(http.RoundTripper) http.RoundTripper {
			return roundTripFunc(func(request *http.Request) (*http.Response, error) {
				body := `{"apiVersion":"v1","kind":"PodList","items":[]}`
				if request.URL.Path == "/apis/apiextensions.k8s.io/v1/customresourcedefinitions" {
					body = `{"apiVersion":"apiextensions.k8s.io/v1","kind":"CustomResourceDefinitionList","items":[]}`
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(body)),
					Request:    request,
				}, nil
			})
		},
	}
	inClusterClient = nil
	t.Cleanup(func() {
		baseConfig = previousConfig
		inClusterClient = previousClient
	})

	cacheFlag := pflag.Lookup("cache-enabled")
	previousCache := cacheFlag.Value.String()
	if err := cacheFlag.Value.Set("true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cacheFlag.Value.Set(previousCache) })
	anonymousFlag := pflag.Lookup("anonymous-admin")
	previousAnonymous := anonymousFlag.Value.String()
	if err := anonymousFlag.Value.Set("false"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = anonymousFlag.Value.Set(previousAnonymous) })

	request := httptest.NewRequest(http.MethodGet, "https://dashboard.example.test", nil)
	request.Header.Set("Cache-Control", "no-cache")
	SetAuthorizationHeader(request, "cache-test-token")
	kubeClient, err := Client(request)
	if err != nil {
		t.Fatalf("Client() error = %v", err)
	}
	if _, err := kubeClient.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{}); err != nil {
		t.Fatalf("cached pod list error = %v", err)
	}

	extensionsClient, err := APIExtensionsClient(request)
	if err != nil {
		t.Fatalf("APIExtensionsClient() error = %v", err)
	}
	if _, err := extensionsClient.ApiextensionsV1().CustomResourceDefinitions().List(context.Background(), metav1.ListOptions{}); err != nil {
		t.Fatalf("cached CRD list error = %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (in roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return in(request)
}
