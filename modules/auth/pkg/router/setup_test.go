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

package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRouterAndV1AreConfigured(t *testing.T) {
	engine := Router()
	if engine == nil || V1() == nil {
		t.Fatal("router or versioned route group is nil")
	}
	engine.GET("/router-test/panic", func(*gin.Context) { panic("test panic") })
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/router-test/panic", nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("recovery middleware status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
}

func TestNewRouterUsesReleaseModeOutsideDevelopment(t *testing.T) {
	previousMode := gin.Mode()
	t.Cleanup(func() { gin.SetMode(previousMode) })

	engine, versioned := newRouter(false)
	if engine == nil || versioned == nil || gin.Mode() != gin.ReleaseMode {
		t.Fatalf("newRouter(false) = %v, %v; Gin mode = %q", engine, versioned, gin.Mode())
	}
}
