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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/pflag"
	v1 "k8s.io/api/authorization/v1"
	"k8s.io/client-go/kubernetes"
	dashboardclient "k8s.io/dashboard/client"
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

func TestSettingsMutationHandlersSaveAndDelete(t *testing.T) {
	cacheFlag := pflag.Lookup("cache-enabled")
	previousCache := cacheFlag.Value.String()
	if err := cacheFlag.Value.Set("false"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cacheFlag.Value.Set(previousCache) })
	dashboardclient.Init(dashboardclient.WithMasterUrl("https://kubernetes.example.test"))

	recording := &recordingSettingsManager{}
	previousManager := manager
	manager = recording
	t.Cleanup(func() { manager = previousManager })

	settingsBody := `{"itemsPerPage":25,"defaultNamespace":"staging"}`
	settingsResponse := invokeSettingsMutation(t, handleSettingsGlobalSave, http.MethodPut, "/settings", settingsBody, nil)
	if settingsResponse.Code != http.StatusNoContent {
		t.Fatalf("settings PUT status = %d, want %d: %s", settingsResponse.Code, http.StatusNoContent, settingsResponse.Body.String())
	}
	if recording.savedSettings == nil || recording.savedSettings.ItemsPerPage == nil || *recording.savedSettings.ItemsPerPage != 25 || recording.savedSettings.DefaultNamespace == nil || *recording.savedSettings.DefaultNamespace != "staging" {
		t.Fatalf("saved settings = %#v", recording.savedSettings)
	}

	pinnedBody := `{"kind":"Pod","name":"web","namespace":"default","namespaced":true}`
	pinnedResponse := invokeSettingsMutation(t, handleSettingsSavePinned, http.MethodPut, "/settings/pinnedresources", pinnedBody, nil)
	if pinnedResponse.Code != http.StatusNoContent {
		t.Fatalf("pinned resource PUT status = %d, want %d: %s", pinnedResponse.Code, http.StatusNoContent, pinnedResponse.Body.String())
	}
	if recording.savedPinned == nil || *recording.savedPinned != (PinnedResource{Kind: "Pod", Name: "web", Namespace: "default", Namespaced: true}) {
		t.Fatalf("saved pinned resource = %#v", recording.savedPinned)
	}

	for _, test := range []struct {
		name   string
		params gin.Params
		want   PinnedResource
	}{
		{
			name:   "namespaced resource",
			params: gin.Params{{Key: "kind", Value: "Pod"}, {Key: "nameOrNamespace", Value: "default"}, {Key: "name", Value: "web"}},
			want:   PinnedResource{Kind: "Pod", Name: "web", Namespace: "default"},
		},
		{
			name:   "cluster scoped resource",
			params: gin.Params{{Key: "kind", Value: "Node"}, {Key: "nameOrNamespace", Value: "node-1"}},
			want:   PinnedResource{Kind: "Node", Name: "node-1"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)
			context.Request = httptest.NewRequest(http.MethodDelete, "/settings/pinnedresources", nil)
			context.Request.Header.Set("Authorization", "Bearer settings-test-token")
			context.Params = test.params
			handleSettingsDeletePinned(context)
			if response.Code != http.StatusNoContent {
				t.Fatalf("pinned resource DELETE status = %d, want %d: %s", response.Code, http.StatusNoContent, response.Body.String())
			}
			if recording.deletedPinned == nil || *recording.deletedPinned != test.want {
				t.Fatalf("deleted pinned resource = %#v, want %#v", recording.deletedPinned, test.want)
			}
		})
	}
}

func TestSettingsMutationHandlersReportBindingAndManagerErrors(t *testing.T) {
	cacheFlag := pflag.Lookup("cache-enabled")
	previousCache := cacheFlag.Value.String()
	if err := cacheFlag.Value.Set("false"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cacheFlag.Value.Set(previousCache) })
	dashboardclient.Init(dashboardclient.WithMasterUrl("https://kubernetes.example.test"))

	previousManager := manager
	defer func() { manager = previousManager }()

	manager = &recordingSettingsManager{}
	for _, test := range []struct {
		name    string
		handler gin.HandlerFunc
		method  string
		path    string
		body    string
		params  gin.Params
	}{
		{name: "invalid settings JSON", handler: handleSettingsGlobalSave, method: http.MethodPut, path: "/settings", body: "{"},
		{name: "invalid pinned resource JSON", handler: handleSettingsSavePinned, method: http.MethodPut, path: "/settings/pinnedresources", body: "{"},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := invokeSettingsMutation(t, test.handler, test.method, test.path, test.body, test.params)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusBadRequest, response.Body.String())
			}
		})
	}

	manager = &recordingSettingsManager{
		saveSettingsErr: errors.New("settings save rejected"),
		savePinnedErr:   errors.New("pinned save rejected"),
		deletePinnedErr: errors.New("pinned delete rejected"),
	}
	for _, test := range []struct {
		name    string
		handler gin.HandlerFunc
		method  string
		path    string
		body    string
		params  gin.Params
	}{
		{name: "settings manager error", handler: handleSettingsGlobalSave, method: http.MethodPut, path: "/settings", body: `{"itemsPerPage":25}`},
		{name: "pinned resource manager error", handler: handleSettingsSavePinned, method: http.MethodPut, path: "/settings/pinnedresources", body: `{"kind":"Pod","name":"web"}`},
		{name: "delete manager error", handler: handleSettingsDeletePinned, method: http.MethodDelete, path: "/settings/pinnedresources/pod/default/web", params: gin.Params{{Key: "kind", Value: "pod"}, {Key: "nameOrNamespace", Value: "default"}, {Key: "name", Value: "web"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := invokeSettingsMutation(t, test.handler, test.method, test.path, test.body, test.params)
			if response.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusInternalServerError, response.Body.String())
			}
		})
	}
}

type recordingSettingsManager struct {
	savedSettings   *Settings
	savedPinned     *PinnedResource
	deletedPinned   *PinnedResource
	saveSettingsErr error
	savePinnedErr   error
	deletePinnedErr error
}

func (in *recordingSettingsManager) GetGlobalSettings(kubernetes.Interface) *Settings {
	return nil
}

func (in *recordingSettingsManager) SaveGlobalSettings(_ kubernetes.Interface, settings *Settings) error {
	in.savedSettings = settings
	return in.saveSettingsErr
}

func (in *recordingSettingsManager) GetPinnedResources(kubernetes.Interface) []PinnedResource {
	return nil
}

func (in *recordingSettingsManager) SavePinnedResource(_ kubernetes.Interface, resource *PinnedResource) error {
	in.savedPinned = resource
	return in.savePinnedErr
}

func (in *recordingSettingsManager) DeletePinnedResource(_ kubernetes.Interface, resource *PinnedResource) error {
	copy := *resource
	in.deletedPinned = &copy
	return in.deletePinnedErr
}

func invokeSettingsMutation(t *testing.T, handler gin.HandlerFunc, method, path, body string, params gin.Params) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Request.Header.Set("Authorization", "Bearer settings-test-token")
	context.Params = params
	handler(context)
	return response
}
