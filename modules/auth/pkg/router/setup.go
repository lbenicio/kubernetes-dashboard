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

package router

import (
	"github.com/gin-gonic/gin"

	"k8s.io/dashboard/auth/pkg/environment"
	"k8s.io/dashboard/csrf"
	"k8s.io/dashboard/helpers"
)

var (
	router   *gin.Engine
	v1       *gin.RouterGroup
	v1NoCSRF *gin.RouterGroup
)

func init() {
	if !environment.IsDev() {
		gin.SetMode(gin.ReleaseMode)
	}

	router = gin.New()
	router.Use(gin.Recovery())
	_ = router.SetTrustedProxies(nil)

	// Routes under this group have CSRF protection.
	v1 = router.Group("/api/v1")
	v1.Use(csrf.Gin().CSRF(
		csrf.Gin().WithCSRFActionGetter(helpers.GetResourceFromPath),
	))

	// Routes under this group skip CSRF (e.g., OIDC exchange for unauthenticated users).
	v1NoCSRF = router.Group("/api/v1")
}

func V1() *gin.RouterGroup {
	return v1
}

// V1NoCSRF returns a route group without CSRF protection.
// Use for endpoints that must be accessible to unauthenticated users.
func V1NoCSRF() *gin.RouterGroup {
	return v1NoCSRF
}

func Router() *gin.Engine {
	return router
}
