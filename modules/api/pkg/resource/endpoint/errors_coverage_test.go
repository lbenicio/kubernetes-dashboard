// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package endpoint

import (
	"errors"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestGetEndpointsReturnsKubernetesListFailure(t *testing.T) {
	wantErr := errors.New("endpoint slice list failed")
	client := fake.NewSimpleClientset()
	client.PrependReactor("list", "endpointslices", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, wantErr
	})
	if _, err := GetEndpoints(client, "default", "web"); !errors.Is(err, wantErr) {
		t.Fatalf("GetEndpoints() error = %v, want %v", err, wantErr)
	}
	if _, err := GetServiceEndpoints(client, "default", "web"); !errors.Is(err, wantErr) {
		t.Fatalf("GetServiceEndpoints() error = %v, want %v", err, wantErr)
	}
}
