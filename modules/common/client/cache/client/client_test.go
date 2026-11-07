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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (in roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return in(request)
}
