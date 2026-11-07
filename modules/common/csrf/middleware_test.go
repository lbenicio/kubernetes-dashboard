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

package csrf

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/emicklei/go-restful/v3"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/xsrftoken"
)

func initializeTestCSRFKey(t *testing.T) {
	t.Helper()
	Ensure()
}

func TestGinCSRFMiddleware(t *testing.T) {
	initializeTestCSRFKey(t)
	previousMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(previousMode) })

	router := gin.New()
	router.Use(Gin().CSRF())
	router.POST("/protected", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.GET("/protected", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	unauthorized := httptest.NewRecorder()
	router.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodPost, "/protected", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("POST without a CSRF token status = %d, want %d", unauthorized.Code, http.StatusUnauthorized)
	}

	valid := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/protected", nil)
	request.Header.Set(csrfTokenHeader, xsrftoken.Generate(Key(), "none", "/protected"))
	router.ServeHTTP(valid, request)
	if valid.Code != http.StatusNoContent {
		t.Errorf("POST with a valid CSRF token status = %d, want %d", valid.Code, http.StatusNoContent)
	}

	get := httptest.NewRecorder()
	router.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/protected", nil))
	if get.Code != http.StatusNoContent {
		t.Errorf("GET status = %d, want %d", get.Code, http.StatusNoContent)
	}
}

func TestGinCSRFMiddlewareOptions(t *testing.T) {
	initializeTestCSRFKey(t)
	previousMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(previousMode) })

	customAction := "custom-action"
	router := gin.New()
	router.Use(Gin().CSRF(
		Gin().WithCSRFActionGetter(func(string) *string { return &customAction }),
		Gin().WithCSRFRunCondition(func(*http.Request) bool { return false }),
	))
	router.POST("/protected", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/protected", nil))
	if response.Code != http.StatusNoContent {
		t.Errorf("request skipped by the configured run condition status = %d, want %d", response.Code, http.StatusNoContent)
	}

	nilActionRouter := gin.New()
	nilActionRouter.Use(Gin().CSRF(Gin().WithCSRFActionGetter(func(string) *string { return nil })))
	nilActionRouter.POST("/protected", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	nilActionResponse := httptest.NewRecorder()
	nilActionRouter.ServeHTTP(nilActionResponse, httptest.NewRequest(http.MethodPost, "/protected", nil))
	if nilActionResponse.Code != http.StatusUnauthorized {
		t.Errorf("request with a nil action status = %d, want %d", nilActionResponse.Code, http.StatusUnauthorized)
	}
}

func TestGoRestfulCSRFMiddleware(t *testing.T) {
	initializeTestCSRFKey(t)

	var action string
	filter := GoRestful().CSRF(
		GoRestful().WithCSRFActionGetter(func(route string) *string {
			action = route
			return &action
		}),
		GoRestful().WithCSRFRunCondition(func(request *restful.Request) bool {
			return request.Request.Method == http.MethodPost
		}),
	)
	container := newGoRestfulTestContainer(filter)

	unauthorized := httptest.NewRecorder()
	container.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodPost, "/test/action", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("POST without a CSRF token status = %d, want %d", unauthorized.Code, http.StatusUnauthorized)
	}

	valid := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/test/action", nil)
	request.Header.Set(csrfTokenHeader, xsrftoken.Generate(Key(), "none", action))
	container.ServeHTTP(valid, request)
	if valid.Code != http.StatusNoContent {
		t.Errorf("POST with a valid CSRF token status = %d, want %d", valid.Code, http.StatusNoContent)
	}

	get := httptest.NewRecorder()
	container.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/test/action", nil))
	if get.Code != http.StatusNoContent {
		t.Errorf("GET status = %d, want %d", get.Code, http.StatusNoContent)
	}
}

func TestGoRestfulCSRFMiddlewareRejectsNilAction(t *testing.T) {
	initializeTestCSRFKey(t)
	filter := GoRestful().CSRF(
		GoRestful().WithCSRFActionGetter(func(string) *string { return nil }),
		GoRestful().WithCSRFRunCondition(func(*restful.Request) bool { return true }),
	)
	response := httptest.NewRecorder()
	newGoRestfulTestContainer(filter).ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/test/action", nil))
	if response.Code != http.StatusUnauthorized {
		t.Errorf("request with a nil action status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestGoRestfulCSRFMiddlewareWithoutOptionsPassesThrough(t *testing.T) {
	initializeTestCSRFKey(t)
	response := httptest.NewRecorder()
	newGoRestfulTestContainer(GoRestful().CSRF()).ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/test/action", nil))
	if response.Code != http.StatusNoContent {
		t.Errorf("request using pass-through middleware status = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func newGoRestfulTestContainer(filter restful.FilterFunction) *restful.Container {
	container := restful.NewContainer()
	service := new(restful.WebService).Path("/test")
	service.Filter(filter)
	service.Route(service.POST("/action").To(func(_ *restful.Request, response *restful.Response) {
		response.WriteHeader(http.StatusNoContent)
	}))
	service.Route(service.GET("/action").To(func(_ *restful.Request, response *restful.Response) {
		response.WriteHeader(http.StatusNoContent)
	}))
	container.Add(service)
	return container
}
