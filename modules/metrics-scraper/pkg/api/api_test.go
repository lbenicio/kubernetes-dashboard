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

package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

func TestManagerRoutesFallbackAndDashboardPrefix(t *testing.T) {
	router := mux.NewRouter()
	Manager(router, nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz?status=ready", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "URL: /healthz?status=ready") {
		t.Fatalf("fallback response = %d %q", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/unknown", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "URL: /api/v1/dashboard/unknown") {
		t.Fatalf("dashboard fallback response = %d %q", response.Code, response.Body.String())
	}
}

func TestDefaultHandlerLogsWriteFailure(t *testing.T) {
	response := failingWriter{}
	DefaultHandler(&response, httptest.NewRequest(http.MethodGet, "/", nil))
	if !errors.Is(response.err, errWriteFailure) {
		t.Fatalf("write error = %v, want %v", response.err, errWriteFailure)
	}
}

var errWriteFailure = errors.New("response write failed")

type failingWriter struct {
	err error
}

func (*failingWriter) Header() http.Header { return make(http.Header) }
func (*failingWriter) WriteHeader(int)     {}
func (in *failingWriter) Write([]byte) (int, error) {
	in.err = errWriteFailure
	return 0, in.err
}
