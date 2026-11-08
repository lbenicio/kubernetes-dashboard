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
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package login

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
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

func TestMeReturnsAuthenticatedBearerUserAfterDiscovery(t *testing.T) {
	setAnonymousAdminForMeTest(t, false)
	token := unsignedJWT(t, map[string]interface{}{
		"kubernetes.io": map[string]interface{}{
			"serviceaccount": map[string]interface{}{"name": "dashboard", "uid": "uid-123"},
		},
	})
	initializeMeKubernetesClient(t, "Bearer "+token)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	user, status, err := me(request, httptest.NewRecorder())
	if err != nil || status != http.StatusOK {
		t.Fatalf("me() = (%#v, %d, %v), want an authenticated response", user, status, err)
	}
	if user.Name != "dashboard" || user.AuthMethod != "header" || !user.Authenticated {
		t.Fatalf("me() user = %#v", user)
	}
}

func TestAnonymousMeReturnsAdministratorAfterDiscovery(t *testing.T) {
	setAnonymousAdminForMeTest(t, true)
	initializeMeKubernetesClient(t, "")
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.AddCookie(&http.Cookie{Name: "kd-session-0", Value: "stale"})
	request.AddCookie(&http.Cookie{Name: "kd-oidc-state", Value: "stale"})
	response := httptest.NewRecorder()

	user, status, err := me(request, response)
	if err != nil || status != http.StatusOK {
		t.Fatalf("anonymous me() = (%#v, %d, %v), want success", user, status, err)
	}
	if user.Name != "Administrator" || user.AuthMethod != "anonymous" || !user.Authenticated {
		t.Fatalf("anonymous me() user = %#v", user)
	}
	assertMeCookiesCleared(t, response)
}

func TestMeReportsKubernetesDiscoveryFailures(t *testing.T) {
	for _, test := range []struct {
		name          string
		anonymous     bool
		authorization string
	}{
		{name: "anonymous administrator", anonymous: true},
		{name: "token user", authorization: "Bearer invalid-token"},
	} {
		t.Run(test.name, func(t *testing.T) {
			setAnonymousAdminForMeTest(t, test.anonymous)
			initializeMeKubernetesClientWithResponse(t, test.authorization, http.StatusForbidden,
				`{"kind":"Status","status":"Failure","message":"discovery denied","reason":"Forbidden","code":403}`)
			request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
			if test.authorization != "" {
				request.Header.Set("Authorization", test.authorization)
			}
			user, status, err := me(request, httptest.NewRecorder())
			if err == nil || status != http.StatusForbidden || user != nil {
				t.Fatalf("me() = (%#v, %d, %v), want forbidden discovery response", user, status, err)
			}
		})
	}
}

func TestOIDCMeReturnsSessionProfileAfterDiscovery(t *testing.T) {
	setAnonymousAdminForMeTest(t, false)
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("o", 32)))
	if err := client.ConfigureSessionKey(key); err != nil {
		t.Fatalf("ConfigureSessionKey() error = %v", err)
	}
	initializeMeKubernetesClient(t, "Bearer verified-id-token")
	seedRequest := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	seedResponse := httptest.NewRecorder()
	session := client.BrowserSession{
		OIDC:            true,
		KubernetesToken: "verified-id-token",
		Name:            "Alice Example",
		Email:           "alice@example.test",
		Picture:         "https://images.example.test/alice.png",
		Groups:          []string{"developers"},
		Scopes:          []string{"openid", "profile"},
		Subject:         "subject-1",
		ExpiresAt:       time.Now().Add(time.Hour),
		TokenExpiry:     time.Now().Add(time.Hour),
	}
	if err := client.SetBrowserSessionCookie(seedResponse, seedRequest, session); err != nil {
		t.Fatalf("SetBrowserSessionCookie() error = %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	for _, cookie := range seedResponse.Result().Cookies() {
		request.AddCookie(cookie)
	}

	user, status, err := me(request, httptest.NewRecorder())
	if err != nil || status != http.StatusOK {
		t.Fatalf("OIDC me() = (%#v, %d, %v), want success", user, status, err)
	}
	if user.Name != session.Name || user.Email != session.Email || user.Picture != session.Picture || user.AuthMethod != "oidc" || !user.Authenticated {
		t.Fatalf("OIDC me() user = %#v", user)
	}
	if len(user.Groups) != 1 || user.Groups[0] != "developers" || len(user.Scopes) != 2 {
		t.Fatalf("OIDC groups/scopes = %#v/%#v", user.Groups, user.Scopes)
	}
}

func TestOIDCMeRefreshFailureAndCookieOutcomes(t *testing.T) {
	setAnonymousAdminForMeTest(t, false)
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("r", 32)))
	if err := client.ConfigureSessionKey(key); err != nil {
		t.Fatalf("ConfigureSessionKey() error = %v", err)
	}

	newRequest := func(t *testing.T) *http.Request {
		t.Helper()
		seedRequest := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		seedResponse := httptest.NewRecorder()
		if err := client.SetBrowserSessionCookie(seedResponse, seedRequest, client.BrowserSession{
			OIDC: true, KubernetesToken: "old-token", Subject: "subject-1", ExpiresAt: time.Now().Add(time.Hour), TokenExpiry: time.Now().Add(-time.Minute),
		}); err != nil {
			t.Fatalf("SetBrowserSessionCookie() error = %v", err)
		}
		request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		for _, cookie := range seedResponse.Result().Cookies() {
			request.AddCookie(cookie)
		}
		return request
	}

	t.Run("refresh failure clears the session", func(t *testing.T) {
		response := httptest.NewRecorder()
		user, status, err := meWithRefresh(newRequest(t), response, func(context.Context, *client.BrowserSession) (bool, error) {
			return false, errors.New("refresh rejected")
		})
		if err == nil || status != http.StatusUnauthorized || user != nil {
			t.Fatalf("meWithRefresh(error) = (%#v, %d, %v)", user, status, err)
		}
		if !hasClearedCookie(response, "kd-session-0") {
			t.Fatal("refresh failure did not clear the browser session cookie")
		}
	})

	t.Run("successful refresh persists the new cookie", func(t *testing.T) {
		setAnonymousAdminForMeTest(t, false)
		initializeMeKubernetesClient(t, "Bearer refreshed-token")
		response := httptest.NewRecorder()
		user, status, err := meWithRefresh(newRequest(t), response, func(_ context.Context, session *client.BrowserSession) (bool, error) {
			session.KubernetesToken = "refreshed-token"
			return true, nil
		})
		if err != nil || status != http.StatusOK || user == nil {
			t.Fatalf("meWithRefresh(success) = (%#v, %d, %v)", user, status, err)
		}
		if !hasSetCookie(response, "kd-session-0") {
			t.Fatal("successful refresh did not persist the browser session cookie")
		}
	})

	t.Run("invalid refreshed session returns internal error", func(t *testing.T) {
		response := httptest.NewRecorder()
		user, status, err := meWithRefresh(newRequest(t), response, func(_ context.Context, session *client.BrowserSession) (bool, error) {
			session.ExpiresAt = time.Time{}
			return true, nil
		})
		if err == nil || status != http.StatusInternalServerError || user != nil {
			t.Fatalf("meWithRefresh(invalid session) = (%#v, %d, %v)", user, status, err)
		}
	})
}

func hasClearedCookie(response *httptest.ResponseRecorder, name string) bool {
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == name && cookie.MaxAge < 0 {
			return true
		}
	}
	return false
}

func hasSetCookie(response *httptest.ResponseRecorder, name string) bool {
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == name && cookie.MaxAge >= 0 {
			return true
		}
	}
	return false
}

func TestMeReturnsTokenSessionProfileAndTokenClaimFallback(t *testing.T) {
	setAnonymousAdminForMeTest(t, false)
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("t", 32)))
	if err := client.ConfigureSessionKey(key); err != nil {
		t.Fatalf("ConfigureSessionKey() error = %v", err)
	}
	token := unsignedJWT(t, map[string]interface{}{
		"kubernetes.io": map[string]interface{}{
			"serviceaccount": map[string]interface{}{"name": "dashboard", "uid": "uid-123"},
		},
	})
	initializeMeKubernetesClient(t, "Bearer "+token)

	for _, test := range []struct {
		name     string
		username string
		wantName string
	}{
		{name: "session name", username: "automation", wantName: "automation"},
		{name: "name from service-account token", wantName: "dashboard"},
	} {
		t.Run(test.name, func(t *testing.T) {
			seedRequest := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
			seedResponse := httptest.NewRecorder()
			session := client.BrowserSession{
				KubernetesToken: token, Name: test.username, Email: "automation@example.test",
				Groups: []string{"system:serviceaccounts"}, Scopes: []string{"read"}, ExpiresAt: time.Now().Add(time.Hour),
			}
			if err := client.SetBrowserSessionCookie(seedResponse, seedRequest, session); err != nil {
				t.Fatalf("SetBrowserSessionCookie() error = %v", err)
			}
			request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
			for _, cookie := range seedResponse.Result().Cookies() {
				request.AddCookie(cookie)
			}
			user, status, err := me(request, httptest.NewRecorder())
			if err != nil || status != http.StatusOK {
				t.Fatalf("me() = (%#v, %d, %v), want success", user, status, err)
			}
			if user.Name != test.wantName || user.AuthMethod != "token" || user.Email != session.Email || !user.Authenticated {
				t.Errorf("token session user = %#v", user)
			}
			if len(user.Groups) != 1 || user.Groups[0] != "system:serviceaccounts" || len(user.Scopes) != 1 || user.Scopes[0] != "read" {
				t.Errorf("token session groups/scopes = %#v/%#v", user.Groups, user.Scopes)
			}
		})
	}
}

func TestTranscodeRejectsValuesThatCannotBeEncodedOrDecoded(t *testing.T) {
	var destination struct {
		Name string `json:"name"`
	}
	if transcode(map[string]string{"name": "service"}, &destination) != true || destination.Name != "service" {
		t.Fatalf("transcode() did not decode a valid value: %#v", destination)
	}
	if transcode(make(chan int), &destination) {
		t.Error("transcode() accepted a value that JSON cannot encode")
	}
	var integer int
	if transcode(map[string]string{"value": "not an integer"}, &integer) {
		t.Error("transcode() accepted a value incompatible with its destination")
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

func setAnonymousAdminForMeTest(t *testing.T, enabled bool) {
	t.Helper()
	flag := pflag.Lookup("anonymous-admin")
	previous := flag.Value.String()
	if err := flag.Value.Set(strconv.FormatBool(enabled)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = flag.Value.Set(previous) })
}

func initializeMeKubernetesClient(t *testing.T, wantAuthorization string) {
	initializeMeKubernetesClientWithResponse(t, wantAuthorization, http.StatusOK,
		`{"major":"1","minor":"37","gitVersion":"v1.37.0"}`)
}

func initializeMeKubernetesClientWithResponse(t *testing.T, wantAuthorization string, status int, body string) {
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
			return meRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				if request.URL.Path != "/version" || request.Header.Get("Authorization") != wantAuthorization {
					return &http.Response{
						StatusCode: http.StatusUnauthorized,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(`{"message":"unexpected request"}`)),
						Request:    request,
					}, nil
				}
				return &http.Response{
					StatusCode: status,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(body)),
					Request:    request,
				}, nil
			})
		}),
	)
}

type meRoundTripFunc func(*http.Request) (*http.Response, error)

func (in meRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return in(request)
}

func assertMeCookiesCleared(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
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
