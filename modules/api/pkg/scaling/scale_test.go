// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

package scaling

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

func TestScaleOperationsReturnConfigurationErrors(t *testing.T) {
	badConfig := &rest.Config{Host: "%"}
	if _, err := GetReplicaCounts(badConfig, "deployments", "default", "web"); err == nil {
		t.Fatal("GetReplicaCounts() accepted an empty API server config")
	}
	if _, err := ScaleResource(badConfig, "deployments", "default", "web", "3"); err == nil {
		t.Fatal("ScaleResource() accepted an empty API server config")
	}
}

func TestGetGroupResourceSupportsCoreAndQualifiedResources(t *testing.T) {
	want := schema.GroupResource{Group: "apps", Resource: "deployments"}
	if got := getGroupResource("deployments"); got != want {
		t.Errorf("getGroupResource(core) = %v, want %v", got, want)
	}
	if got := getGroupResource("deployments.apps"); got != want {
		t.Errorf("getGroupResource(qualified) = %v, want %v", got, want)
	}
}
