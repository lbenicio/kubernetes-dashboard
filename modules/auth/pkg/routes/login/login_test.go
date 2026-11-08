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

package login

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	v1 "k8s.io/dashboard/auth/api/v1"
	"k8s.io/dashboard/client"
)

func TestLoginValidatesBearerTokenWithKubernetes(t *testing.T) {
	for _, test := range []struct {
		name       string
		statusCode int
		wantStatus int
	}{
		{name: "valid token", statusCode: http.StatusOK, wantStatus: http.StatusOK},
		{name: "rejected token", statusCode: http.StatusUnauthorized, wantStatus: http.StatusUnauthorized},
	} {
		t.Run(test.name, func(t *testing.T) {
			initializeLoginClient(t, test.statusCode)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/login", nil)
			response, status, err := login(&v1.LoginRequest{Token: "login-test-token"}, request)
			if status != test.wantStatus {
				t.Fatalf("login() status = %d, want %d (error %v)", status, test.wantStatus, err)
			}
			if test.wantStatus == http.StatusOK {
				if err != nil || response == nil || response.Token != "login-test-token" {
					t.Fatalf("login() = %#v, %d, %v", response, status, err)
				}
			} else if err == nil || response != nil {
				t.Fatalf("login() = %#v, %d, %v; want Kubernetes authorization error", response, status, err)
			}
		})
	}
}

func initializeLoginClient(t *testing.T, responseStatus int) {
	t.Helper()
	cacheFlag := pflag.Lookup("cache-enabled")
	previousCache := cacheFlag.Value.String()
	if err := cacheFlag.Value.Set("false"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cacheFlag.Value.Set(previousCache) })

	client.Init(
		client.WithMasterUrl("https://kubernetes.example.test"),
		client.WithWrapTransport(func(http.RoundTripper) http.RoundTripper {
			return loginRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				if request.URL.Path != "/version" || !strings.HasPrefix(request.Header.Get("Authorization"), "Bearer ") {
					return nil, http.ErrHandlerTimeout
				}
				body := `{"major":"1","minor":"37","gitVersion":"v1.37.0"}`
				if responseStatus != http.StatusOK {
					body = `{"apiVersion":"v1","kind":"Status","status":"Failure","message":"token rejected","reason":"Unauthorized","code":401}`
				}
				return &http.Response{
					StatusCode: responseStatus,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(body)),
					Request:    request,
				}, nil
			})
		}),
	)
}

type loginRoundTripFunc func(*http.Request) (*http.Response, error)

func (in loginRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return in(request)
}
