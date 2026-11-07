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

package settings

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	v1 "k8s.io/api/authorization/v1"
	"k8s.io/client-go/kubernetes"
)

func TestToSelfSubjectAccessReview(t *testing.T) {
	review := ToSelfSubjectAccessReview("dashboard", "settings", "ConfigMap", "PATCH")
	want := &v1.SelfSubjectAccessReview{
		Spec: v1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &v1.ResourceAttributes{
				Namespace: "dashboard",
				Name:      "settings",
				Resource:  "configmaps",
				Verb:      "patch",
			},
		},
	}
	if review.Spec.ResourceAttributes == nil || *review.Spec.ResourceAttributes != *want.Spec.ResourceAttributes {
		t.Fatalf("review attributes = %#v, want %#v", review.Spec.ResourceAttributes, want.Spec.ResourceAttributes)
	}
}

func TestSettingsMutationHandlersReportUninitializedClient(t *testing.T) {
	cases := []struct {
		name    string
		handler gin.HandlerFunc
		method  string
		path    string
		body    string
	}{
		{name: "save settings", handler: handleSettingsGlobalSave, method: http.MethodPut, path: "/settings", body: `{}`},
		{name: "save pinned resource", handler: handleSettingsSavePinned, method: http.MethodPut, path: "/settings/pinnedresources", body: `{}`},
		{name: "delete pinned resource", handler: handleSettingsDeletePinned, method: http.MethodDelete, path: "/settings/pinnedresources/pod/default/web"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			tc.handler(context)
			if recorder.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
			}
		})
	}
}

func TestSettingsCanIHandlerReturnsDeniedWithoutInitializedClient(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/settings/cani", nil)

	handleGetSettingsGlobalCanI(context)

	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"allowed":false`) {
		t.Fatalf("CanI response = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestSettingsReadHandlersReturnManagerValues(t *testing.T) {
	previousManager := manager
	manager = fixedSettingsManager{
		settings: &Settings{ItemsPerPage: intPointer(25)},
		pinned:   []PinnedResource{{Kind: "Pod", Name: "web", Namespace: "default", Namespaced: true}},
	}
	t.Cleanup(func() { manager = previousManager })

	settingsResponse := invokeSettingsHandler(handleGetSettingGlobal, http.MethodGet, "/settings")
	if settingsResponse.Code != http.StatusOK {
		t.Fatalf("settings GET status = %d, want %d", settingsResponse.Code, http.StatusOK)
	}
	var settings Settings
	if err := json.Unmarshal(settingsResponse.Body.Bytes(), &settings); err != nil {
		t.Fatalf("decode settings response: %v", err)
	}
	if settings.ItemsPerPage == nil || *settings.ItemsPerPage != 25 {
		t.Fatalf("settings response = %#v", settings)
	}

	pinnedResponse := invokeSettingsHandler(handleSettingsGetPinned, http.MethodGet, "/settings/pinnedresources")
	if pinnedResponse.Code != http.StatusOK {
		t.Fatalf("pinned resources GET status = %d, want %d", pinnedResponse.Code, http.StatusOK)
	}
	var pinned []PinnedResource
	if err := json.Unmarshal(pinnedResponse.Body.Bytes(), &pinned); err != nil {
		t.Fatalf("decode pinned resources response: %v", err)
	}
	if len(pinned) != 1 || pinned[0].Name != "web" || pinned[0].Namespace != "default" {
		t.Fatalf("pinned resources response = %#v", pinned)
	}
}

type fixedSettingsManager struct {
	settings *Settings
	pinned   []PinnedResource
}

func (in fixedSettingsManager) GetGlobalSettings(kubernetes.Interface) *Settings {
	return in.settings
}

func (in fixedSettingsManager) SaveGlobalSettings(kubernetes.Interface, *Settings) error {
	return nil
}

func (in fixedSettingsManager) GetPinnedResources(kubernetes.Interface) []PinnedResource {
	return in.pinned
}

func (in fixedSettingsManager) SavePinnedResource(kubernetes.Interface, *PinnedResource) error {
	return nil
}

func (in fixedSettingsManager) DeletePinnedResource(kubernetes.Interface, *PinnedResource) error {
	return nil
}

func invokeSettingsHandler(handler gin.HandlerFunc, method, path string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(method, path, nil)
	handler(context)
	return response
}
