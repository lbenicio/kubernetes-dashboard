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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHandleMeReturnsAuthenticatedUserAfterDiscovery(t *testing.T) {
	setAnonymousAdminForMeTest(t, false)
	initializeMeKubernetesClient(t, "Bearer me-handler-test-token")
	previousMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(previousMode) })

	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	context.Request.Header.Set("Authorization", "Bearer me-handler-test-token")
	handleMe(context)
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("handleMe() status/header = %d/%q: %s", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
	}
	var user map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &user); err != nil {
		t.Fatalf("decode handleMe() response: %v", err)
	}
	if user["authMethod"] != "header" || user["authenticated"] != true {
		t.Errorf("handleMe() user = %#v", user)
	}
}
