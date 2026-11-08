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
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRouterAccessors(t *testing.T) {
	if Root() == nil {
		t.Fatal("Root() returned nil")
	}
	if Router() == nil {
		t.Fatal("Router() returned nil")
	}
}

func TestNewRouterSelectsReleaseModeOutsideDevelopment(t *testing.T) {
	previousMode := gin.Mode()
	t.Cleanup(func() { gin.SetMode(previousMode) })

	engine, root := newRouter(false)
	if engine == nil || root == nil || gin.Mode() != gin.ReleaseMode {
		t.Fatalf("newRouter(false) = %v, %v; Gin mode = %q", engine, root, gin.Mode())
	}

	gin.SetMode(gin.DebugMode)
	if engine, root = newRouter(true); engine == nil || root == nil || gin.Mode() != gin.DebugMode {
		t.Fatalf("newRouter(true) = %v, %v; Gin mode = %q", engine, root, gin.Mode())
	}
}
