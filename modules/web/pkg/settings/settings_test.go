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

import "testing"

func TestSettingsDefault(t *testing.T) {
	defaults := (*Settings)(nil).Default()
	if defaults.ItemsPerPage == nil || *defaults.ItemsPerPage != 10 {
		t.Fatalf("default items per page = %v, want 10", defaults.ItemsPerPage)
	}
	if defaults.DefaultNamespace == nil || *defaults.DefaultNamespace != "default" {
		t.Fatalf("default namespace = %v, want default", defaults.DefaultNamespace)
	}

	custom := &Settings{ItemsPerPage: intPointer(25), ClusterName: stringPointer("test-cluster")}
	if got := custom.Default(); got != custom {
		t.Fatal("Default() should return the receiver when it is non-nil")
	}
	if *custom.ItemsPerPage != 25 || *custom.ClusterName != "test-cluster" {
		t.Fatalf("Default() overwrote custom values: %#v", custom)
	}
	if custom.DefaultNamespace == nil || *custom.DefaultNamespace != "default" {
		t.Fatalf("Default() did not fill missing values: %#v", custom)
	}
}

func TestSettingsJSONRoundTripAndErrors(t *testing.T) {
	settings := &Settings{ItemsPerPage: intPointer(50), DefaultNamespace: stringPointer("staging")}
	parsed, err := UnmarshalSettings(settings.Marshal())
	if err != nil {
		t.Fatalf("UnmarshalSettings() error = %v", err)
	}
	if parsed.ItemsPerPage == nil || *parsed.ItemsPerPage != 50 || parsed.DefaultNamespace == nil || *parsed.DefaultNamespace != "staging" {
		t.Fatalf("round trip = %#v", parsed)
	}
	if _, err := UnmarshalSettings("{"); err == nil {
		t.Fatal("UnmarshalSettings() accepted malformed JSON")
	}
}

func intPointer(value int) *int { return &value }

func stringPointer(value string) *string { return &value }
