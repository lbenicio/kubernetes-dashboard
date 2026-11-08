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
	"k8s.io/dashboard/api/pkg/resource/common"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestHorizontalPodAutoscalerListReadersReturnKubernetesFailures(t *testing.T) {
	for _, test := range []struct {
		name string
		call func(*fake.Clientset) error
	}{
		{name: "all namespaces", call: func(client *fake.Clientset) error {
			_, err := GetHorizontalPodAutoscalerList(client, common.NewNamespaceQuery(nil), dataselect.NoDataSelect)
			return err
		}},
		{name: "target resource", call: func(client *fake.Clientset) error {
			_, err := GetHorizontalPodAutoscalerListForResource(client, "default", "deployment", "web")
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			wantErr := errors.New("horizontal pod autoscaler list failed")
			client := fake.NewSimpleClientset()
			client.PrependReactor("list", "horizontalpodautoscalers", func(ktesting.Action) (bool, runtime.Object, error) {
				return true, nil, wantErr
			})
			if err := test.call(client); !errors.Is(err, wantErr) {
				t.Fatalf("HPA list error = %v; want %v", err, wantErr)
			}
		})
	}
}
