// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package deployment

import (
	"errors"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/dashboard/api/pkg/resource/common"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestGetDeploymentListReturnsFailuresFromEveryResourceChannel(t *testing.T) {
	for _, resource := range []string{"deployments", "pods", "events", "replicasets"} {
		t.Run(resource, func(t *testing.T) {
			wantErr := errors.New(resource + " list failed")
			client := fake.NewSimpleClientset()
			client.PrependReactor("list", resource, func(ktesting.Action) (bool, runtime.Object, error) {
				return true, nil, wantErr
			})
			if _, err := GetDeploymentList(client, common.NewNamespaceQuery(nil), dataselect.NoDataSelect, nil); !errors.Is(err, wantErr) {
				t.Fatalf("GetDeploymentList() error = %v; want %v", err, wantErr)
			}
		})
	}
}
