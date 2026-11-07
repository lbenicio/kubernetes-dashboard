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

package oidc

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBeginLoginReturnsNotFoundWhenOIDCIsDisabled(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/oidc/login", nil)

	beginLogin(context)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("OIDC login status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestCompleteLoginReturnsNotFoundWhenOIDCIsDisabled(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/oidc/callback?state=state&code=code", nil)

	completeLogin(context)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("OIDC callback status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}
