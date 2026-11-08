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
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/pflag"

	"k8s.io/dashboard/client"
)

func TestLoginHandlersCoverConfigurationCookiesAndErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if err := client.ConfigureSessionKey(base64.StdEncoding.EncodeToString([]byte(strings.Repeat("l", 32)))); err != nil {
		t.Fatalf("ConfigureSessionKey() error = %v", err)
	}
	initializeLoginClient(t, http.StatusOK)

	setAnonymousAdminForLoginHandlerTest(t, false)
	t.Run("valid login stores token in protected cookie", func(t *testing.T) {
		response := runLoginHandler(t, `{"token":"login-test-token"}`)
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("login handler status/header = %d/%q; body=%s", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode login response: %v", err)
		}
		if token, exposed := body["token"]; !exposed || token != "" {
			t.Errorf("login response token = %#v; want an empty token after session-cookie creation", body["token"])
		}
		if len(response.Result().Cookies()) == 0 {
			t.Fatal("login response did not set a protected browser session cookie")
		}
	})

	t.Run("malformed request", func(t *testing.T) {
		response := runLoginHandler(t, "{")
		if response.Code != http.StatusBadRequest || response.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("malformed login status/header = %d/%q; body=%s", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
		}
	})

	t.Run("kubernetes rejects token", func(t *testing.T) {
		initializeLoginClient(t, http.StatusUnauthorized)
		response := runLoginHandler(t, `{"token":"login-test-token"}`)
		if response.Code != http.StatusUnauthorized {
			t.Errorf("rejected login status = %d; want 401: %s", response.Code, response.Body.String())
		}
		initializeLoginClient(t, http.StatusOK)
	})

	t.Run("oversized browser session reports cookie error", func(t *testing.T) {
		response := runLoginHandler(t, `{"token":"`+strings.Repeat("x", 20000)+`"}`)
		if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "Could not create a secure login session") {
			t.Errorf("oversized session response = %d %s", response.Code, response.Body.String())
		}
	})

	t.Run("anonymous admin hides token login", func(t *testing.T) {
		setAnonymousAdminForLoginHandlerTest(t, true)
		response := runLoginHandler(t, `{"token":"login-test-token"}`)
		if response.Code != http.StatusNotFound {
			t.Errorf("anonymous login status = %d; want 404", response.Code)
		}
		setAnonymousAdminForLoginHandlerTest(t, false)
	})

	t.Run("config and logout disable caching and clear cookies", func(t *testing.T) {
		configRecorder := httptest.NewRecorder()
		configContext, _ := gin.CreateTestContext(configRecorder)
		configContext.Request = httptest.NewRequest(http.MethodGet, "/api/v1/login/config", nil)
		handleLoginConfig(configContext)
		if configRecorder.Code != http.StatusOK || configRecorder.Header().Get("Cache-Control") != "no-store" ||
			!strings.Contains(configRecorder.Body.String(), `"anonymousAdmin":false`) {
			t.Errorf("login config response = %d %q %s", configRecorder.Code, configRecorder.Header().Get("Cache-Control"), configRecorder.Body.String())
		}

		logoutRecorder := httptest.NewRecorder()
		logoutContext, _ := gin.CreateTestContext(logoutRecorder)
		logoutContext.Request = httptest.NewRequest(http.MethodPost, "/api/v1/logout", nil)
		logoutContext.Request.AddCookie(&http.Cookie{Name: "kd-session-0", Value: "stale"})
		logoutContext.Request.AddCookie(&http.Cookie{Name: "kd-oidc-state", Value: "stale"})
		handleLogout(logoutContext)
		logoutContext.Writer.WriteHeaderNow()
		if logoutContext.Writer.Status() != http.StatusNoContent || logoutRecorder.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("logout status/header = %d/%q", logoutContext.Writer.Status(), logoutRecorder.Header().Get("Cache-Control"))
		}
		cleared := map[string]bool{}
		for _, cookie := range logoutRecorder.Result().Cookies() {
			if cookie.MaxAge < 0 {
				cleared[cookie.Name] = true
			}
		}
		if !cleared["kd-session-0"] || !cleared["kd-oidc-state"] {
			t.Errorf("logout did not clear both session cookies: %v", cleared)
		}
	})
}

func runLoginHandler(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/login", strings.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	handleLogin(context)
	return response
}

func setAnonymousAdminForLoginHandlerTest(t *testing.T, enabled bool) {
	t.Helper()
	flag := pflag.Lookup("anonymous-admin")
	if flag == nil {
		t.Fatal("anonymous-admin flag is not registered")
	}
	previous := flag.Value.String()
	if err := flag.Value.Set(strconv.FormatBool(enabled)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = flag.Value.Set(previous) })
}
