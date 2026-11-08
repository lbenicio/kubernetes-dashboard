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
	"context"
	"encoding/json"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"

	"k8s.io/dashboard/web/pkg/args"
)

func newSettingsTestClient(t *testing.T, data map[string]string) *fake.Clientset {
	t.Helper()
	client := fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: args.SettingsConfigMapName(), Namespace: args.Namespace()},
		Data:       data,
	})
	return client
}

func TestSettingsManagerLoadsConfiguredValuesAndPinnedResources(t *testing.T) {
	settings := `{"itemsPerPage":25,"defaultNamespace":"staging"}`
	pinned := `[{"kind":"Pod","name":"web","namespace":"staging","namespaced":true}]`
	client := newSettingsTestClient(t, map[string]string{ConfigMapSettingsKey: settings, PinnedResourcesKey: pinned})
	manager := NewSettingsManager()

	loaded := manager.GetGlobalSettings(client)
	if loaded.ItemsPerPage == nil || *loaded.ItemsPerPage != 25 || loaded.DefaultNamespace == nil || *loaded.DefaultNamespace != "staging" {
		t.Fatalf("loaded settings = %#v", loaded)
	}
	if got := manager.GetPinnedResources(client); len(got) != 1 || got[0].Name != "web" {
		t.Fatalf("loaded pinned resources = %#v", got)
	}
}

func TestSettingsManagerReturnsDefaultsWhenConfigMapIsUnavailable(t *testing.T) {
	client := fake.NewSimpleClientset()
	manager := NewSettingsManager()

	got := manager.GetGlobalSettings(client)
	if got.ItemsPerPage == nil || *got.ItemsPerPage != 10 {
		t.Fatalf("default settings = %#v", got)
	}
	if pinned := manager.GetPinnedResources(client); len(pinned) != 0 {
		t.Fatalf("pinned resources = %#v, want empty", pinned)
	}
}

func TestSettingsManagerSaveSettingsAndDetectConcurrentChanges(t *testing.T) {
	client := newSettingsTestClient(t, nil)
	manager := NewSettingsManager()
	input := &Settings{ItemsPerPage: intPointer(20)}

	if err := manager.SaveGlobalSettings(client, input); err != nil {
		t.Fatalf("SaveGlobalSettings() error = %v", err)
	}
	configMap, err := client.CoreV1().ConfigMaps(args.Namespace()).Get(context.Background(), args.SettingsConfigMapName(), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get saved ConfigMap: %v", err)
	}
	var saved Settings
	if err := json.Unmarshal([]byte(configMap.Data[ConfigMapSettingsKey]), &saved); err != nil {
		t.Fatalf("decode saved settings: %v", err)
	}
	if saved.ItemsPerPage == nil || *saved.ItemsPerPage != 20 || saved.DefaultNamespace == nil || *saved.DefaultNamespace != "default" {
		t.Fatalf("saved settings = %#v", saved)
	}

	manager = NewSettingsManager()
	manager.GetGlobalSettings(client)
	configMap.Data[ConfigMapSettingsKey] = `{"itemsPerPage":30}`
	if _, err := client.CoreV1().ConfigMaps(args.Namespace()).Update(context.Background(), configMap, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("update ConfigMap: %v", err)
	}
	if err := manager.SaveGlobalSettings(client, &Settings{}); err == nil || err.Error() != ConcurrentSettingsChangeError {
		t.Fatalf("SaveGlobalSettings() concurrent error = %v, want %q", err, ConcurrentSettingsChangeError)
	}
}

func TestSettingsManagerSaveAndDeletePinnedResources(t *testing.T) {
	client := newSettingsTestClient(t, nil)
	manager := NewSettingsManager()
	pinned := &PinnedResource{Kind: "Pod", Name: "web", Namespace: "default", Namespaced: true}

	if err := manager.SavePinnedResource(client, pinned); err != nil {
		t.Fatalf("SavePinnedResource() error = %v", err)
	}
	if err := manager.SavePinnedResource(client, pinned); err != nil {
		t.Fatalf("saving a duplicate pinned resource should be a no-op: %v", err)
	}
	if got := manager.GetPinnedResources(client); len(got) != 1 || got[0].Name != "web" {
		t.Fatalf("pinned resources after save = %#v", got)
	}
	if err := manager.DeletePinnedResource(client, pinned); err != nil {
		t.Fatalf("DeletePinnedResource() error = %v", err)
	}
	if got := manager.GetPinnedResources(client); len(got) != 0 {
		t.Fatalf("pinned resources after delete = %#v", got)
	}
	if err := manager.DeletePinnedResource(client, pinned); err == nil || err.Error() != PinnedResourceNotFoundError {
		t.Fatalf("deleting absent pinned resource error = %v, want %q", err, PinnedResourceNotFoundError)
	}
}

func TestSettingsManagerReportsConcurrentChangeBeforeSavingPinnedResource(t *testing.T) {
	client := newSettingsTestClient(t, nil)
	manager := NewSettingsManager()
	manager.GetPinnedResources(client)

	configMap, err := client.CoreV1().ConfigMaps(args.Namespace()).Get(context.Background(), args.SettingsConfigMapName(), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get ConfigMap: %v", err)
	}
	configMap.Data = map[string]string{PinnedResourcesKey: `[]`}
	if _, err := client.CoreV1().ConfigMaps(args.Namespace()).Update(context.Background(), configMap, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("update ConfigMap: %v", err)
	}

	err = manager.SavePinnedResource(client, &PinnedResource{Kind: "Pod", Name: "web"})
	if err == nil || err.Error() != ConcurrentSettingsChangeError {
		t.Fatalf("SavePinnedResource() concurrent error = %v, want %q", err, ConcurrentSettingsChangeError)
	}
}

func TestSettingsManagerReportsConcurrentChangeBeforeDeletingPinnedResource(t *testing.T) {
	client := newSettingsTestClient(t, nil)
	manager := NewSettingsManager()
	pinned := &PinnedResource{Kind: "Pod", Name: "web", Namespace: "default", Namespaced: true}
	if err := manager.SavePinnedResource(client, pinned); err != nil {
		t.Fatalf("initial SavePinnedResource() error = %v", err)
	}

	configMap, err := client.CoreV1().ConfigMaps(args.Namespace()).Get(context.Background(), args.SettingsConfigMapName(), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get ConfigMap: %v", err)
	}
	configMap.Data[PinnedResourcesKey] = `[]`
	if _, err := client.CoreV1().ConfigMaps(args.Namespace()).Update(context.Background(), configMap, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("update ConfigMap: %v", err)
	}

	err = manager.DeletePinnedResource(client, pinned)
	if err == nil || err.Error() != ConcurrentSettingsChangeError {
		t.Fatalf("DeletePinnedResource() concurrent error = %v, want %q", err, ConcurrentSettingsChangeError)
	}
}

func TestSettingsManagerHandlesConfigMapLoadErrors(t *testing.T) {
	client := newSettingsTestClient(t, nil)
	client.PrependReactor("get", "configmaps", func(_ ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, context.DeadlineExceeded
	})
	manager := NewSettingsManager()

	if got := manager.GetGlobalSettings(client); got.DefaultNamespace == nil || *got.DefaultNamespace != "default" {
		t.Fatalf("settings after load error = %#v", got)
	}
}

func TestSettingsManagerIgnoresMalformedConfigMapValues(t *testing.T) {
	client := newSettingsTestClient(t, map[string]string{
		ConfigMapSettingsKey: "{",
		PinnedResourcesKey:   "{",
	})
	manager := NewSettingsManager()
	if got := manager.GetGlobalSettings(client); got.ItemsPerPage == nil || *got.ItemsPerPage != 10 {
		t.Fatalf("settings after malformed values = %#v, want defaults", got)
	}
	if got := manager.GetPinnedResources(client); len(got) != 0 {
		t.Fatalf("pinned resources after malformed values = %#v, want empty", got)
	}
}

func TestSettingsManagerReturnsConfigMapPatchErrors(t *testing.T) {
	client := newSettingsTestClient(t, nil)
	client.PrependReactor("patch", "configmaps", func(_ ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("patch rejected")
	})
	manager := NewSettingsManager()

	if err := manager.SaveGlobalSettings(client, &Settings{}); err == nil || err.Error() != "patch rejected" {
		t.Fatalf("SaveGlobalSettings() patch error = %v, want patch rejected", err)
	}
}
