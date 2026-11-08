// Copyright 2017 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sidecar

import (
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachinery "k8s.io/apimachinery/pkg/types"

	metricapi "k8s.io/dashboard/api/pkg/integration/metric/api"
	"k8s.io/dashboard/types"
)

func TestGetSidecarSelector(t *testing.T) {
	resource1 := map[string]string{
		"resource": "1",
	}
	resource2 := map[string]string{
		"resource": "2",
	}
	var cachedPodList = []v1.Pod{
		{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      "1",
				Labels:    resource1,
				Namespace: "a",
			},
		},
		{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      "2",
				Labels:    resource2,
				Namespace: "a",
			},
		},
		{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      "3",
				Labels:    resource1,
				Namespace: "a",
			},
		},
		{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      "4",
				Labels:    resource1,
				Namespace: "b",
			},
		},
		{
			ObjectMeta: metaV1.ObjectMeta{
				Name:   "5",
				Labels: resource1,
			},
		},
	}
	testCases := []struct {
		Info                   string
		ResourceSelector       metricapi.ResourceSelector
		ExpectedPath           string
		ExpectedTargetResource types.ResourceKind
		ExpectedResources      []string
	}{
		{
			"ResourceSelector for native resource - pod",
			metricapi.ResourceSelector{
				Namespace:    "bar",
				ResourceType: types.ResourceKindPod,
				ResourceName: "foo",
			},
			`namespaces/bar/pod-list/`,
			types.ResourceKindPod,
			[]string{"foo"},
		},
		{
			"ResourceSelector for native resource - node",
			metricapi.ResourceSelector{
				Namespace:    "barn",
				ResourceType: types.ResourceKindNode,
				ResourceName: "foon",
			},
			`nodes/`,
			types.ResourceKindNode,
			[]string{"foon"},
		},
		{
			"ResourceSelector for derived resource with old style selector",
			metricapi.ResourceSelector{
				Namespace:    "a",
				ResourceType: types.ResourceKindDeployment,
				ResourceName: "baba",
				Selector:     resource1,
			},
			`namespaces/a/pod-list/`,
			types.ResourceKindPod,
			[]string{"1", "3"},
		},
	}
	for _, testCase := range testCases {
		sel, err := getSidecarSelector(testCase.ResourceSelector,
			&metricapi.CachedResources{Pods: cachedPodList})
		if err != nil {
			t.Errorf("Test Case: %s. Failed to get SidecarSelector. - %s", testCase.Info, err)
			return
		}
		if !reflect.DeepEqual(sel.Resources, testCase.ExpectedResources) {
			t.Errorf("Test Case: %s. Converted resource selector to incorrect native resources. Got %v, expected %v.",
				testCase.Info, sel.Resources, testCase.ExpectedResources)
		}
		if sel.TargetResourceType != testCase.ExpectedTargetResource {
			t.Errorf("Test Case: %s. Used invalid target resource type. Got %s, expected %s.",
				testCase.Info, sel.TargetResourceType, testCase.ExpectedTargetResource)
		}
		if sel.Path != testCase.ExpectedPath {
			t.Errorf("Test Case: %s. Converted to invalid sidecar download path. Got %s, expected %s.",
				testCase.Info, sel.Path, testCase.ExpectedPath)
		}

	}
}

func TestGetMyPodsFromCacheSelectsDeploymentPodsByNamespaceAndLabels(t *testing.T) {
	selector := metricapi.ResourceSelector{
		Namespace:    "apps",
		ResourceType: types.ResourceKindDeployment,
		Selector:     map[string]string{"app": "dashboard"},
	}
	pods := []v1.Pod{
		{ObjectMeta: metaV1.ObjectMeta{Name: "dashboard-a", Namespace: "apps", Labels: map[string]string{"app": "dashboard"}, UID: "uid-a"}},
		{ObjectMeta: metaV1.ObjectMeta{Name: "dashboard-b", Namespace: "apps", Labels: map[string]string{"app": "dashboard", "tier": "frontend"}, UID: "uid-b"}},
		{ObjectMeta: metaV1.ObjectMeta{Name: "other-app", Namespace: "apps", Labels: map[string]string{"app": "other"}, UID: "uid-c"}},
		{ObjectMeta: metaV1.ObjectMeta{Name: "other-namespace", Namespace: "default", Labels: map[string]string{"app": "dashboard"}, UID: "uid-d"}},
	}

	got, err := getMyPodsFromCache(selector, pods)
	if err != nil {
		t.Fatalf("getMyPodsFromCache() error = %v", err)
	}
	if names := podListToNameList(got); !reflect.DeepEqual(names, []string{"dashboard-a", "dashboard-b"}) {
		t.Fatalf("matching pods = %v, want dashboard-a and dashboard-b", names)
	}
	if uids := podListToUIDList(got); !reflect.DeepEqual(uids, []apimachinery.UID{"uid-a", "uid-b"}) {
		t.Fatalf("matching pod UIDs = %v, want uid-a and uid-b", uids)
	}
}

func TestGetMyPodsFromCacheSelectsControllerOwnedPodsAndReportsMissingCache(t *testing.T) {
	controller := true
	notController := false
	selector := metricapi.ResourceSelector{
		Namespace:    "apps",
		ResourceType: types.ResourceKindReplicaSet,
		UID:          "owner-uid",
	}
	pods := []v1.Pod{
		{ObjectMeta: metaV1.ObjectMeta{Name: "owned", Namespace: "apps", UID: "pod-uid", OwnerReferences: []metaV1.OwnerReference{{UID: "owner-uid", Controller: &controller}}}},
		{ObjectMeta: metaV1.ObjectMeta{Name: "not-controller", Namespace: "apps", OwnerReferences: []metaV1.OwnerReference{{UID: "owner-uid", Controller: &notController}}}},
		{ObjectMeta: metaV1.ObjectMeta{Name: "different-owner", Namespace: "apps", OwnerReferences: []metaV1.OwnerReference{{UID: "other-uid", Controller: &controller}}}},
		{ObjectMeta: metaV1.ObjectMeta{Name: "different-namespace", Namespace: "default", OwnerReferences: []metaV1.OwnerReference{{UID: "owner-uid", Controller: &controller}}}},
	}

	got, err := getMyPodsFromCache(selector, pods)
	if err != nil || !reflect.DeepEqual(podListToNameList(got), []string{"owned"}) {
		t.Fatalf("getMyPodsFromCache() = %v, %v; want only owned pod", podListToNameList(got), err)
	}
	if _, err := getMyPodsFromCache(selector, nil); err == nil {
		t.Fatal("getMyPodsFromCache() accepted a missing pod cache")
	}
}

func TestGetSidecarSelectorRejectsUnsupportedDerivedNativeResource(t *testing.T) {
	unsupportedKind := types.ResourceKind("test-derived-resource")
	previous, existed := metricapi.DerivedResources[unsupportedKind]
	metricapi.DerivedResources[unsupportedKind] = types.ResourceKindNode
	t.Cleanup(func() {
		if existed {
			metricapi.DerivedResources[unsupportedKind] = previous
		} else {
			delete(metricapi.DerivedResources, unsupportedKind)
		}
	})

	if _, err := getSidecarSelector(metricapi.ResourceSelector{ResourceType: unsupportedKind}, nil); err == nil {
		t.Fatal("getSidecarSelector() accepted a derived resource that maps to an unsupported native type")
	}
}

func TestGetSidecarSelectorsPreservesPositionsForUnsupportedResources(t *testing.T) {
	selectors := []metricapi.ResourceSelector{
		{Namespace: "apps", ResourceType: types.ResourceKindPod, ResourceName: "pod-a", UID: "pod-uid"},
		{Namespace: "apps", ResourceType: types.ResourceKindService, ResourceName: "service-a"},
		{Namespace: "apps", ResourceType: types.ResourceKindDeployment, ResourceName: "deployment-a"},
	}
	got := getSidecarSelectors(selectors, &metricapi.CachedResources{})
	if len(got) != len(selectors) {
		t.Fatalf("getSidecarSelectors() returned %d entries, want %d", len(got), len(selectors))
	}
	if got[0].Path != "namespaces/apps/pod-list/" || !reflect.DeepEqual(got[0].Resources, []string{"pod-a"}) ||
		!reflect.DeepEqual(got[0].Label[types.ResourceKindPod], []apimachinery.UID{"pod-uid"}) {
		t.Errorf("native pod selector = %#v", got[0])
	}
	if got[1].Path != "" || got[1].TargetResourceType != "" {
		t.Errorf("unsupported service selector should leave an empty slot: %#v", got[1])
	}
	if got[2].Path != "" || got[2].TargetResourceType != "" {
		t.Errorf("derived selector without cached pods should leave an empty slot: %#v", got[2])
	}
}
