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

package extensions_test

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	extensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"k8s.io/dashboard/client/cache"
	"k8s.io/dashboard/client/cache/client/common"
	extensionsclient "k8s.io/dashboard/client/cache/client/extensions"
)

func TestCachedCustomResourceDefinitionLister(t *testing.T) {
	client, err := extensionsclient.NewClient(testRESTConfig(), nil, common.CachedClientOptions{Token: "extensions-client-test-token"})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	cache.Clear[extensionsv1.CustomResourceDefinitionList]()
	list, err := client.CustomResourceDefinitions().List(context.Background(), metav1.ListOptions{})
	if err != nil || list == nil || len(list.Items) != 0 {
		t.Fatalf("CustomResourceDefinitions().List() = %#v, %v", list, err)
	}
}

func TestNewClientRejectsMalformedRESTConfig(t *testing.T) {
	if _, err := extensionsclient.NewClient(&rest.Config{Host: "http://%"}, nil, common.CachedClientOptions{}); err == nil {
		t.Fatal("NewClient() accepted a malformed REST config")
	}
}

func TestNewClientReturnsTransportConfigurationErrors(t *testing.T) {
	missingCertificate := filepath.Join(t.TempDir(), "missing.crt")
	config := &rest.Config{
		Host: "https://kubernetes.example.test",
		TLSClientConfig: rest.TLSClientConfig{
			CertFile: missingCertificate,
			KeyFile:  missingCertificate,
		},
	}
	if _, err := extensionsclient.NewClient(config, nil, common.CachedClientOptions{}); err == nil {
		t.Fatal("NewClient() accepted missing TLS client certificate files")
	}
}

func testRESTConfig() *rest.Config {
	return &rest.Config{
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
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (in roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return in(request)
}
