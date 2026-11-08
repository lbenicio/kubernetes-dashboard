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

package client_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	extensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"k8s.io/dashboard/client/cache"
	cacheclient "k8s.io/dashboard/client/cache/client"
	"k8s.io/dashboard/client/cache/client/common"
)

func TestNewCachedClientPerformsCoreListThroughTransport(t *testing.T) {
	cache.Clear[corev1.PodList]()
	userAgent := make(chan string, 1)
	config := &rest.Config{
		Host: "https://kubernetes.example.test",
		WrapTransport: func(http.RoundTripper) http.RoundTripper {
			return roundTripFunc(func(request *http.Request) (*http.Response, error) {
				userAgent <- request.UserAgent()
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"items":[]}`)),
					Request:    request,
				}, nil
			})
		},
	}
	client, err := cacheclient.New(config, common.CachedClientOptions{Token: "clientset-test-token"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	list, err := client.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{})
	if err != nil || list == nil || len(list.Items) != 0 {
		t.Fatalf("cached pod list = %#v, %v", list, err)
	}
	select {
	case got := <-userAgent:
		if got == "" {
			t.Error("cached client did not set a User-Agent")
		}
	default:
		t.Error("cached client did not issue a request")
	}
}

func TestNewCachedExtensionsClientWrapsTypedAndDiscoveryClients(t *testing.T) {
	cache.Clear[extensionsv1.CustomResourceDefinitionList]()
	config := &rest.Config{
		Host: "https://kubernetes.example.test",
		WrapTransport: func(http.RoundTripper) http.RoundTripper {
			return roundTripFunc(func(request *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"items":[]}`)),
					Request:    request,
				}, nil
			})
		},
	}
	client, err := cacheclient.NewCachedExtensionsClient(config, nil, common.CachedClientOptions{Token: "extensions-client-test-token"})
	if err != nil {
		t.Fatalf("NewCachedExtensionsClient() error = %v", err)
	}
	if client.ApiextensionsV1() == nil || client.Discovery() == nil {
		t.Fatal("cached extensions client did not expose its typed API and discovery clients")
	}
	definitions, err := client.ApiextensionsV1().CustomResourceDefinitions().List(context.Background(), metav1.ListOptions{})
	if err != nil || definitions == nil || len(definitions.Items) != 0 {
		t.Fatalf("CustomResourceDefinitions().List() = %#v, %v", definitions, err)
	}
}

func TestCachedClientConstructorsRejectMalformedRESTConfig(t *testing.T) {
	config := &rest.Config{Host: "http://%"}
	if _, err := cacheclient.New(config, common.CachedClientOptions{}); err == nil {
		t.Error("New() accepted a malformed REST config")
	}
	if _, err := cacheclient.NewCachedExtensionsClient(config, nil, common.CachedClientOptions{}); err == nil {
		t.Error("NewCachedExtensionsClient() accepted a malformed REST config")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (in roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return in(request)
}
