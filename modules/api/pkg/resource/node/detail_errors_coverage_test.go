// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package node

import (
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestGetNodeDetailReturnsCriticalKubernetesFailures(t *testing.T) {
	query := dataselect.NoDataSelect
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-a"}}
	tests := []struct {
		name   string
		client func() *fake.Clientset
	}{
		{
			name: "node get",
			client: func() *fake.Clientset {
				return fake.NewSimpleClientset()
			},
		},
		{
			name: "first pod list",
			client: func() *fake.Clientset {
				client := fake.NewSimpleClientset(node)
				client.PrependReactor("list", "pods", func(clienttesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("first pod list failed")
				})
				return client
			},
		},
		{
			name: "second pod list",
			client: func() *fake.Clientset {
				client := fake.NewSimpleClientset(node)
				lists := 0
				client.PrependReactor("list", "pods", func(clienttesting.Action) (bool, runtime.Object, error) {
					lists++
					if lists == 2 {
						return true, nil, errors.New("second pod list failed")
					}
					return false, nil, nil
				})
				return client
			},
		},
		{
			name: "node event search",
			client: func() *fake.Clientset {
				client := fake.NewSimpleClientset(node)
				searches := 0
				client.PrependReactor("list", "events", func(clienttesting.Action) (bool, runtime.Object, error) {
					searches++
					if searches == 2 {
						return true, nil, errors.New("node event search failed")
					}
					return false, nil, nil
				})
				return client
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := GetNodeDetail(test.client(), nil, node.Name, query); err == nil {
				t.Fatal("GetNodeDetail() succeeded after a critical Kubernetes request failed")
			}
		})
	}
}
