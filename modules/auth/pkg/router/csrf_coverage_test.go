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

package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"k8s.io/dashboard/csrf"
)

func TestRouterCSRFAllowsOIDCCallbackAndRejectsOtherPosts(t *testing.T) {
	csrf.Ensure()
	V1().POST("/csrf-coverage", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	V1().POST("/oidc/callback", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	for _, test := range []struct {
		path string
		want int
	}{
		{path: "/api/v1/csrf-coverage", want: http.StatusUnauthorized},
		{path: "/api/v1/oidc/callback", want: http.StatusNoContent},
	} {
		t.Run(test.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, test.path, nil)
			Router().ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("POST %s status = %d; want %d", test.path, response.Code, test.want)
			}
		})
	}
}
