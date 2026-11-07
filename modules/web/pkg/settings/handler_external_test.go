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

package settings_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"k8s.io/dashboard/client"
	"k8s.io/dashboard/web/pkg/router"
	_ "k8s.io/dashboard/web/pkg/settings"
)

func TestSettingsMutationRoutesRejectMalformedJSONAfterClientSetup(t *testing.T) {
	cacheFlag := pflag.Lookup("cache-enabled")
	previousCache := cacheFlag.Value.String()
	if err := cacheFlag.Value.Set("false"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cacheFlag.Value.Set(previousCache) })
	client.Init(client.WithMasterUrl("https://kubernetes.example.test"))

	for _, test := range []struct {
		name string
		path string
	}{
		{name: "global settings", path: "/settings"},
		{name: "pinned resource", path: "/settings/pinnedresources"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPut, test.path, strings.NewReader("{"))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer settings-test-token")
			response := httptest.NewRecorder()
			router.Router().ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("PUT %s status = %d, want %d: %s", test.path, response.Code, http.StatusBadRequest, response.Body.String())
			}
		})
	}
}
