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

package client

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAuthorizationHeaderHelpers(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "https://dashboard.example.test/", nil)
	if HasAuthorizationHeader(request) || GetBearerToken(request) != "" {
		t.Fatal("request without authentication unexpectedly had a token")
	}

	request.Header.Set("Authorization", "Basic abc")
	if HasAuthorizationHeader(request) || GetBearerToken(request) != "" {
		t.Fatal("non-bearer authorization value was accepted")
	}
	request.Header.Set("Authorization", "Bearer ")
	if HasAuthorizationHeader(request) {
		t.Fatal("empty bearer token was accepted")
	}

	SetAuthorizationHeader(request, "signed-token")
	if !HasAuthorizationHeader(request) || GetBearerToken(request) != "signed-token" {
		t.Fatalf("bearer header helpers returned %q", GetBearerToken(request))
	}
}

func TestAuthorizationHelpersReadEncryptedBrowserSession(t *testing.T) {
	configureTestSessionKey(t)
	request := httptest.NewRequest(http.MethodGet, "https://dashboard.example.test/", nil)
	recorder := httptest.NewRecorder()
	session := BrowserSession{KubernetesToken: "session-token", ExpiresAt: time.Now().Add(time.Minute)}
	if err := SetBrowserSessionCookie(recorder, request, session); err != nil {
		t.Fatalf("SetBrowserSessionCookie() error = %v", err)
	}
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Value != "" {
			request.AddCookie(cookie)
		}
	}

	if !HasAuthorizationHeader(request) || GetBearerToken(request) != session.KubernetesToken {
		t.Fatalf("session authorization helpers returned token %q", GetBearerToken(request))
	}
}
