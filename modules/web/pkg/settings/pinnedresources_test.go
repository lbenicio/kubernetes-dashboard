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
	"reflect"
	"testing"
)

func TestPinnedResourceEqualityAndLookup(t *testing.T) {
	resource := &PinnedResource{Kind: "Pod", Name: "web", Namespace: "default", DisplayName: "frontend"}
	matching := &PinnedResource{Kind: "Pod", Name: "web", Namespace: "default", DisplayName: "renamed"}
	other := &PinnedResource{Kind: "Pod", Name: "web", Namespace: "other"}
	resources := PinnedResources{*resource, {Kind: "Service", Name: "api"}}

	if !resource.Equals(matching) {
		t.Fatal("Equals should compare kind, name, and namespace, not display name")
	}
	if resources.IndexOf(matching) != 0 || !resources.Includes(matching) {
		t.Fatal("matching pinned resource was not found")
	}
	if resources.IndexOf(other) != -1 || resources.Includes(other) {
		t.Fatal("resource from another namespace unexpectedly matched")
	}
}

func TestPinnedResourcesDelete(t *testing.T) {
	resources := PinnedResources{
		{Kind: "Pod", Name: "web", Namespace: "default"},
		{Kind: "Service", Name: "api"},
		{Kind: "Pod", Name: "worker", Namespace: "default"},
	}

	if got, want := clonePinnedResources(resources).DeleteAt(1, 1), (PinnedResources{resources[0], resources[2]}); !reflect.DeepEqual(got, want) {
		t.Fatalf("DeleteAt middle = %#v, want %#v", got, want)
	}
	if got, want := clonePinnedResources(resources).DeleteAt(1, 2), (PinnedResources{resources[0]}); !reflect.DeepEqual(got, want) {
		t.Fatalf("DeleteAt through end = %#v, want %#v", got, want)
	}
	if got := resources.DeleteAt(-1, 1); !reflect.DeepEqual(got, resources) {
		t.Fatalf("DeleteAt negative index changed resources: %#v", got)
	}
	if got := resources.DeleteAt(len(resources)+1, 1); !reflect.DeepEqual(got, resources) {
		t.Fatalf("DeleteAt out-of-range index changed resources: %#v", got)
	}
	if got := clonePinnedResources(resources).Delete(&PinnedResource{Kind: "Service", Name: "api"}); !reflect.DeepEqual(got, (PinnedResources{resources[0], resources[2]})) {
		t.Fatalf("Delete = %#v", got)
	}
	if got := resources.Delete(&PinnedResource{Kind: "ConfigMap", Name: "settings"}); !reflect.DeepEqual(got, resources) {
		t.Fatalf("deleting a missing resource changed resources: %#v", got)
	}
}

func clonePinnedResources(resources PinnedResources) PinnedResources {
	return append(PinnedResources(nil), resources...)
}

func TestPinnedResourcesJSONRoundTripAndErrors(t *testing.T) {
	resources := PinnedResources{{Kind: "Pod", Name: "web", Namespace: "default", Namespaced: true}}
	data := resources.Marshal()
	parsed, err := UnmarshalPinnedResources(data)
	if err != nil {
		t.Fatalf("UnmarshalPinnedResources() error = %v", err)
	}
	if !reflect.DeepEqual(*parsed, []PinnedResource(resources)) {
		t.Fatalf("round trip = %#v, want %#v", *parsed, resources)
	}
	if _, err := UnmarshalPinnedResources("{"); err == nil {
		t.Fatal("UnmarshalPinnedResources() accepted malformed JSON")
	}
}
