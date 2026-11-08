// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package storageclass

import (
	"errors"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestGetStorageClassListReturnsClientFailure(t *testing.T) {
	wantErr := errors.New("storage class list failed")
	client := fake.NewSimpleClientset()
	client.PrependReactor("list", "storageclasses", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, wantErr
	})
	if _, err := GetStorageClassList(client, dataselect.NoDataSelect); !errors.Is(err, wantErr) {
		t.Fatalf("GetStorageClassList() error = %v; want %v", err, wantErr)
	}
}
