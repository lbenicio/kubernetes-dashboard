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

package customresourcedefinition

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/rest"
	clienttesting "k8s.io/client-go/testing"

	"k8s.io/dashboard/api/pkg/resource/common"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func crdClientWithAPIVersion(apiVersion string) *fake.Clientset {
	client := fake.NewSimpleClientset()
	if apiVersion != "" {
		client.Discovery().(*fakediscovery.FakeDiscovery).Resources = []*metaV1.APIResourceList{{GroupVersion: groupName + "/" + apiVersion}}
	}
	return client
}

func TestGetExtensionsAPIVersion(t *testing.T) {
	t.Run("returns preferred supported group version", func(t *testing.T) {
		version, err := GetExtensionsAPIVersion(crdClientWithAPIVersion(v1))
		if err != nil || version != v1 {
			t.Fatalf("GetExtensionsAPIVersion() = %q, %v; want %q, nil", version, err, v1)
		}
	})

	t.Run("returns not found when group is absent", func(t *testing.T) {
		if _, err := GetExtensionsAPIVersion(crdClientWithAPIVersion("")); err == nil {
			t.Fatal("GetExtensionsAPIVersion() error = nil; want not found")
		}
	})

	t.Run("returns discovery errors", func(t *testing.T) {
		client := crdClientWithAPIVersion("")
		wantErr := fmt.Errorf("discovery unavailable")
		client.PrependReactor("get", "group", func(clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, wantErr
		})
		if _, err := GetExtensionsAPIVersion(client); !errors.Is(err, wantErr) {
			t.Fatalf("GetExtensionsAPIVersion() error = %v; want %v", err, wantErr)
		}
	})
}

func TestCustomResourceDefinitionHelpersRejectUnsupportedVersion(t *testing.T) {
	client := crdClientWithAPIVersion("v1beta1")
	tests := []struct {
		name string
		call func() error
	}{
		{name: "list definitions", call: func() error {
			_, err := GetCustomResourceDefinitionList(client, dataselect.DefaultDataSelect)
			return err
		}},
		{name: "definition detail", call: func() error { _, err := GetCustomResourceDefinitionDetail(client, nil, "example.test"); return err }},
		{name: "list custom objects", call: func() error {
			_, err := GetCustomResourceObjectList(client, nil, &common.NamespaceQuery{}, dataselect.DefaultDataSelect, "examples.test")
			return err
		}},
		{name: "custom object detail", call: func() error {
			_, err := GetCustomResourceObjectDetail(client, &common.NamespaceQuery{}, nil, "examples.test", "one")
			return err
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if err == nil || !strings.Contains(err.Error(), "unsupported extensions api") {
				t.Fatalf("call() error = %v; want unsupported API version", err)
			}
		})
	}
}

func TestCustomResourceDefinitionHelpersPropagateDiscoveryErrors(t *testing.T) {
	tests := []struct {
		name string
		call func(*fake.Clientset) error
	}{
		{name: "list definitions", call: func(client *fake.Clientset) error {
			_, err := GetCustomResourceDefinitionList(client, dataselect.DefaultDataSelect)
			return err
		}},
		{name: "definition detail", call: func(client *fake.Clientset) error {
			_, err := GetCustomResourceDefinitionDetail(client, nil, "example.test")
			return err
		}},
		{name: "list custom objects", call: func(client *fake.Clientset) error {
			_, err := GetCustomResourceObjectList(client, nil, &common.NamespaceQuery{}, dataselect.DefaultDataSelect, "examples.test")
			return err
		}},
		{name: "custom object detail", call: func(client *fake.Clientset) error {
			_, err := GetCustomResourceObjectDetail(client, &common.NamespaceQuery{}, nil, "examples.test", "one")
			return err
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := crdClientWithAPIVersion("")
			wantErr := fmt.Errorf("discovery unavailable")
			client.PrependReactor("get", "group", func(clienttesting.Action) (bool, runtime.Object, error) {
				return true, nil, wantErr
			})
			if err := tc.call(client); !errors.Is(err, wantErr) {
				t.Fatalf("call() error = %v; want %v", err, wantErr)
			}
		})
	}
}

func TestCustomResourceDefinitionHelpersDelegateV1Requests(t *testing.T) {
	client := crdClientWithAPIVersion(v1)
	if _, err := GetCustomResourceDefinitionList(client, dataselect.DefaultDataSelect); err != nil {
		t.Fatalf("GetCustomResourceDefinitionList() error = %v", err)
	}

	// Invalid config keeps these dynamic-client delegates offline while checking
	// that supported-version dispatch reaches the v1 implementations.
	invalidConfig := &rest.Config{Host: "%"}
	if _, err := GetCustomResourceDefinitionDetail(client, invalidConfig, "example.test"); err == nil {
		t.Error("GetCustomResourceDefinitionDetail() accepted an invalid REST config")
	}
	if _, err := GetCustomResourceObjectList(client, invalidConfig, &common.NamespaceQuery{}, dataselect.DefaultDataSelect, "examples.test"); err == nil {
		t.Error("GetCustomResourceObjectList() accepted an invalid REST config")
	}
	if _, err := GetCustomResourceObjectDetail(client, &common.NamespaceQuery{}, invalidConfig, "examples.test", "one"); err == nil {
		t.Error("GetCustomResourceObjectDetail() accepted an invalid REST config")
	}
}
