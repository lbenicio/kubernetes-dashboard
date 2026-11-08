// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package customresourcedefinition

import (
	"errors"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakediscovery "k8s.io/client-go/discovery/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestCustomResourceHelpersReportDiscoveryFailuresAndUnsupportedVersions(t *testing.T) {
	t.Run("discovery failure", func(t *testing.T) {
		client := apiextensionsfake.NewSimpleClientset()
		client.Discovery().(*fakediscovery.FakeDiscovery).PrependReactor("get", "", func(ktesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("discovery failed")
		})
		if _, err := GetCustomResourceDefinitionList(client, nil); err == nil {
			t.Fatal("GetCustomResourceDefinitionList() returned no error after discovery failure")
		}
	})

	for _, test := range []struct {
		name      string
		resources []*metav1.APIResourceList
	}{
		{name: "missing extensions group"},
		{name: "unsupported version", resources: []*metav1.APIResourceList{{GroupVersion: apiextensionsv1.GroupName + "/v2"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := apiextensionsfake.NewSimpleClientset()
			client.Discovery().(*fakediscovery.FakeDiscovery).Resources = test.resources
			if _, err := GetCustomResourceDefinitionList(client, nil); err == nil {
				t.Fatal("GetCustomResourceDefinitionList() accepted an unsupported API version")
			}
		})
	}
}
