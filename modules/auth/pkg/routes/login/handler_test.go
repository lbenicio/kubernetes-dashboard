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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/pflag"
)

func TestHandleLoginConfig(t *testing.T) {
	context, response := newGinTestContext(http.MethodGet, "/api/v1/login/config", "")
	handleLoginConfig(context)

	if response.Code != http.StatusOK {
		t.Fatalf("login config status = %d, want %d", response.Code, http.StatusOK)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", response.Header().Get("Cache-Control"))
	}
	var result map[string]bool
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode login config response: %v", err)
	}
	if _, ok := result["oidcEnabled"]; !ok {
		t.Error("login config response is missing oidcEnabled")
	}
	if _, ok := result["anonymousAdmin"]; !ok {
		t.Error("login config response is missing anonymousAdmin")
	}
}

func TestHandleLoginRejectsMalformedRequest(t *testing.T) {
	context, response := newGinTestContext(http.MethodPost, "/api/v1/login", "{")
	context.Request.Header.Set("Content-Type", "application/json")
	handleLogin(context)

	if response.Code != http.StatusBadRequest {
		t.Errorf("malformed login status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", response.Header().Get("Cache-Control"))
	}
}

func TestHandleLoginReturnsKubernetesClientInitializationError(t *testing.T) {
	context, response := newGinTestContext(http.MethodPost, "/api/v1/login", `{"token":"test-token"}`)
	context.Request.Header.Set("Content-Type", "application/json")
	handleLogin(context)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("valid login without initialized client status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", response.Header().Get("Cache-Control"))
	}
}

func TestHandleLoginRejectsAnonymousAdminMode(t *testing.T) {
	flag := pflag.Lookup("anonymous-admin")
	previous := flag.Value.String()
	if err := flag.Value.Set("true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = flag.Value.Set(previous) })
	context, response := newGinTestContext(http.MethodPost, "/api/v1/login", `{"token":"ignored"}`)
	handleLogin(context)
	if response.Code != http.StatusNotFound {
		t.Fatalf("login in anonymous admin mode status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestHandleLogoutClearsBrowserCookies(t *testing.T) {
	context, response := newGinTestContext(http.MethodPost, "/api/v1/logout", "")
	handleLogout(context)

	if context.Writer.Status() != http.StatusNoContent {
		t.Errorf("logout status = %d, want %d", context.Writer.Status(), http.StatusNoContent)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", response.Header().Get("Cache-Control"))
	}
	cookies := response.Result().Cookies()
	if len(cookies) < 2 {
		t.Errorf("logout cleared %d cookies, want the session and OIDC state cookies", len(cookies))
	}
}

func newGinTestContext(method, path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	previousMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	response := httptest.NewRecorder()
	testingContext, _ := gin.CreateTestContext(response)
	gin.SetMode(previousMode)
	testingContext.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	return testingContext, response
}
