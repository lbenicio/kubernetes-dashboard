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

package helpers_test

import (
	"reflect"
	"testing"

	"k8s.io/dashboard/helpers"
)

func TestGetResourceFromPath(t *testing.T) {
	cases := []struct {
		url, expected string
	}{
		{
			"/api/v1/pod",
			"pod",
		},
		{
			"/api/v1/node",
			"node",
		},
		{
			"/api/v1",
			"",
		},
		{
			"/api/v1/",
			"",
		},
		{
			"/other/v1/pod",
			"",
		},
	}
	for _, c := range cases {
		actual := helpers.GetResourceFromPath(c.url)
		if c.expected == "" {
			if actual != nil {
				t.Errorf("GetResourceFromPath(%#v) returns %#v, expected nil", c.url, actual)
			}
		} else if !reflect.DeepEqual(actual, &c.expected) {
			t.Errorf("mapUrlToResource(%#v) returns %#v, expected %#v", c.url, actual, c.expected)
		}
	}
}

func TestEnvironmentRandomAndHashHelpers(t *testing.T) {
	t.Setenv("HELPER_TEST_VALUE", "configured")
	if got := helpers.GetEnv("HELPER_TEST_VALUE", "fallback"); got != "configured" {
		t.Errorf("GetEnv(configured) = %q", got)
	}
	if got := helpers.GetEnv("HELPER_TEST_MISSING", "fallback"); got != "fallback" {
		t.Errorf("GetEnv(missing) = %q, want fallback", got)
	}

	if got := helpers.RandomBytes(0); len(got) != 0 {
		t.Errorf("RandomBytes(0) length = %d", len(got))
	}
	first, second := helpers.RandomBytes(32), helpers.RandomBytes(32)
	if len(first) != 32 || len(second) != 32 {
		t.Fatalf("RandomBytes(32) lengths = %d, %d", len(first), len(second))
	}
	if got := helpers.Random64BaseEncodedBytes(32); len(got) == 0 {
		t.Error("Random64BaseEncodedBytes(32) returned an empty string")
	}

	value := struct{ Name string }{Name: "dashboard"}
	hash, err := helpers.HashObject(value)
	if err != nil || hash == "" {
		t.Fatalf("HashObject() = %q, %v", hash, err)
	}
	if repeated, err := helpers.HashObject(value); err != nil || repeated != hash {
		t.Errorf("HashObject() repeat = %q, %v, want %q", repeated, err, hash)
	}
	if hash, err := helpers.HashObject(make(chan int)); err == nil || hash != "" {
		t.Errorf("HashObject(unmarshalable) = %q, %v", hash, err)
	}
}
