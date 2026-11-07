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
	"golang.org/x/net/xsrftoken"

	"k8s.io/dashboard/csrf"
)

func TestHandleCSRFActionReturnsActionBoundToken(t *testing.T) {
	csrf.Ensure()
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/csrftoken/login", nil)
	context.Params = gin.Params{{Key: "action", Value: "login"}}

	handleCSRFAction(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("CSRF token status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", recorder.Header().Get("Cache-Control"))
	}

	var response csrf.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode CSRF response: %v", err)
	}
	if !xsrftoken.Valid(response.Token, csrf.Key(), "none", "login") {
		t.Errorf("token %q is not valid for the login action", response.Token)
	}
	if xsrftoken.Valid(response.Token, csrf.Key(), "none", "logout") {
		t.Error("login token was also valid for the logout action")
	}
}
