// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package persistentvolume

import (
	"errors"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestGetPersistentVolumeDetailReturnsKubernetesFailure(t *testing.T) {
	wantErr := errors.New("persistent volume request failed")
	client := fake.NewSimpleClientset()
	client.PrependReactor("get", "persistentvolumes", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, wantErr
	})
	if _, err := GetPersistentVolumeDetail(client, "data"); !errors.Is(err, wantErr) {
		t.Fatalf("GetPersistentVolumeDetail() error = %v, want %v", err, wantErr)
	}
}
