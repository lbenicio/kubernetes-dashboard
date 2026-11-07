// Copyright 2026 The Kubernetes Authors.
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

package login

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/spf13/pflag"

	"k8s.io/dashboard/client"
)

func TestGetUserFromToken(t *testing.T) {
	serviceAccountToken, err := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"sub": "service-account-subject",
		"kubernetes.io": map[string]interface{}{
			"serviceaccount": map[string]interface{}{"name": "dashboard", "uid": "uid-123"},
		},
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("create service account token: %v", err)
	}

	cases := []struct {
		name     string
		token    string
		wantName string
	}{
		{name: "nested service account claim", token: serviceAccountToken, wantName: "dashboard"},
		{name: "malformed service account claim", token: unsignedJWT(t, map[string]interface{}{"serviceaccount": 42})},
		{name: "invalid token", token: "not-a-jwt"},
		{name: "missing service account claim", token: unsignedJWT(t, map[string]interface{}{"sub": "user"})},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			user := getUserFromToken(tc.token)
			if !user.Authenticated {
				t.Error("bearer token user is not marked authenticated")
			}
			if user.Name != tc.wantName {
				t.Errorf("user name = %q, want %q", user.Name, tc.wantName)
			}
		})
	}
}

func TestTraverseFindsNestedClaims(t *testing.T) {
	claims := map[string]interface{}{"outer": map[string]interface{}{"target": "found"}}
	found, value := traverse("target", claims)
	if !found || value != "found" {
		t.Errorf("traverse() = (%t, %v), want (true, found)", found, value)
	}

	found, _ = traverse("missing", claims)
	if found {
		t.Error("traverse() found a missing claim")
	}
}

func TestMeReportsKubernetesClientInitializationError(t *testing.T) {
	flag := pflag.Lookup("anonymous-admin")
	previous := flag.Value.String()
	if err := flag.Value.Set("false"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = flag.Value.Set(previous) })

	_, status, err := me(httptest.NewRequest(http.MethodGet, "/api/v1/me", nil), httptest.NewRecorder())
	if err == nil || status != http.StatusInternalServerError {
		t.Fatalf("me() without initialized client = status %d, err %v", status, err)
	}
}

func TestAnonymousMeClearsStaleCookiesBeforeClientCheck(t *testing.T) {
	flag := pflag.Lookup("anonymous-admin")
	previous := flag.Value.String()
	if err := flag.Value.Set("true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = flag.Value.Set(previous) })

	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.AddCookie(&http.Cookie{Name: "kd-session-0", Value: "stale"})
	request.AddCookie(&http.Cookie{Name: "kd-oidc-state", Value: "stale"})
	response := httptest.NewRecorder()
	_, status, err := me(request, response)
	if err == nil || status != http.StatusInternalServerError {
		t.Fatalf("anonymous me() without initialized client = status %d, err %v", status, err)
	}
	cleared := map[string]bool{}
	for _, cookie := range response.Result().Cookies() {
		if cookie.MaxAge < 0 {
			cleared[cookie.Name] = true
		}
	}
	if !cleared["kd-session-0"] || !cleared["kd-oidc-state"] {
		t.Fatalf("stale cookies were not cleared: %v", cleared)
	}
}

func TestOIDCMeUsesSessionTokenForClientRequest(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("m", 32)))
	if err := client.ConfigureSessionKey(key); err != nil {
		t.Fatalf("ConfigureSessionKey() error = %v", err)
	}
	flag := pflag.Lookup("anonymous-admin")
	previous := flag.Value.String()
	if err := flag.Value.Set("false"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = flag.Value.Set(previous) })

	seedRequest := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	seedResponse := httptest.NewRecorder()
	session := client.BrowserSession{OIDC: true, KubernetesToken: "verified-id-token", Subject: "subject-1", ExpiresAt: time.Now().Add(time.Hour)}
	if err := client.SetBrowserSessionCookie(seedResponse, seedRequest, session); err != nil {
		t.Fatalf("SetBrowserSessionCookie() error = %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	for _, cookie := range seedResponse.Result().Cookies() {
		request.AddCookie(cookie)
	}
	_, status, err := me(request, httptest.NewRecorder())
	if err == nil || status != http.StatusInternalServerError {
		t.Fatalf("OIDC me() without initialized client = status %d, err %v", status, err)
	}
}

func unsignedJWT(t *testing.T, claims map[string]interface{}) string {
	t.Helper()
	header, err := json.Marshal(map[string]string{"alg": "none", "typ": "JWT"})
	if err != nil {
		t.Fatalf("marshal JWT header: %v", err)
	}
	body, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal JWT claims: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body) + "."
}
