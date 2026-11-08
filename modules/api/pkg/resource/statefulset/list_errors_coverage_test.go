// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package statefulset

import (
	"errors"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/dashboard/api/pkg/resource/common"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestGetStatefulSetListReturnsPodAndEventFailures(t *testing.T) {
	for _, test := range []struct {
		resource string
	}{
		{resource: "pods"},
		{resource: "events"},
	} {
		t.Run(test.resource, func(t *testing.T) {
			wantErr := errors.New(test.resource + " list failed")
			client := fake.NewSimpleClientset()
			client.PrependReactor("list", test.resource, func(ktesting.Action) (bool, runtime.Object, error) {
				return true, nil, wantErr
			})
			if _, err := GetStatefulSetList(client, common.NewNamespaceQuery(nil), dataselect.NoDataSelect, nil); !errors.Is(err, wantErr) {
				t.Fatalf("GetStatefulSetList() error = %v; want %v", err, wantErr)
			}
		})
	}
}
