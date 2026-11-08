// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package horizontalpodautoscaler

import (
	"errors"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestGetHorizontalPodAutoscalerDetailReturnsKubernetesFailure(t *testing.T) {
	wantErr := errors.New("horizontal pod autoscaler request failed")
	client := fake.NewSimpleClientset()
	client.PrependReactor("get", "horizontalpodautoscalers", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, wantErr
	})
	if _, err := GetHorizontalPodAutoscalerDetail(client, "default", "web"); !errors.Is(err, wantErr) {
		t.Fatalf("GetHorizontalPodAutoscalerDetail() error = %v, want %v", err, wantErr)
	}
}
