// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package event

import (
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	"k8s.io/dashboard/api/pkg/resource/common"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestEventReadersReturnKubernetesListFailures(t *testing.T) {
	tests := []struct {
		name string
		call func(*testing.T)
	}{
		{
			name: "resource events",
			call: func(t *testing.T) {
				client := failingEventListClient()
				if _, err := GetEvents(client, "default", "pod-a"); err == nil {
					t.Fatal("GetEvents() succeeded after the event list failed")
				}
			},
		},
		{
			name: "all pod events",
			call: func(t *testing.T) {
				client := failingEventListClient()
				if _, err := GetPodsEvents(client, corev1.NamespaceAll, nil); err == nil {
					t.Fatal("GetPodsEvents() succeeded after the event list failed")
				}
			},
		},
		{
			name: "pod lookup before events",
			call: func(t *testing.T) {
				client := fake.NewSimpleClientset()
				client.PrependReactor("list", "pods", func(clienttesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("pod list failed")
				})
				if _, err := GetPodEvents(client, "default", "pod-a"); err == nil {
					t.Fatal("GetPodEvents() succeeded after the pod list failed")
				}
			},
		},
		{
			name: "pod event list",
			call: func(t *testing.T) {
				client := fake.NewSimpleClientset()
				client.PrependReactor("list", "events", func(clienttesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("event list failed")
				})
				if _, err := GetPodEvents(client, "default", "pod-a"); err == nil {
					t.Fatal("GetPodEvents() succeeded after the event list failed")
				}
			},
		},
		{
			name: "node lookup",
			call: func(t *testing.T) {
				if _, err := GetNodeEvents(fake.NewSimpleClientset(), dataselect.NoDataSelect, "node-a"); err == nil {
					t.Fatal("GetNodeEvents() succeeded for a missing node")
				}
			},
		},
		{
			name: "node event search",
			call: func(t *testing.T) {
				client := fake.NewSimpleClientset(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-a"}})
				client.PrependReactor("list", "events", func(clienttesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("node event search failed")
				})
				if _, err := GetNodeEvents(client, dataselect.NoDataSelect, "node-a"); err == nil {
					t.Fatal("GetNodeEvents() succeeded after its event search failed")
				}
			},
		},
		{
			name: "second node event search",
			call: func(t *testing.T) {
				client := fake.NewSimpleClientset(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-a"}})
				searches := 0
				client.PrependReactor("list", "events", func(clienttesting.Action) (bool, runtime.Object, error) {
					searches++
					if searches == 2 {
						return true, nil, errors.New("second node event search failed")
					}
					return false, nil, nil
				})
				if _, err := GetNodeEvents(client, dataselect.NoDataSelect, "node-a"); err == nil {
					t.Fatal("GetNodeEvents() succeeded after its second event search failed")
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, test.call)
	}
}

func TestGetEventsRejectsMalformedResourceNameAndNamespaceListFailure(t *testing.T) {
	if _, err := GetEvents(fake.NewSimpleClientset(), "default", "pod-a=other"); err == nil {
		t.Fatal("GetEvents() accepted a resource name that makes its field selector invalid")
	}

	wantErr := errors.New("namespace event list failed")
	client := fake.NewSimpleClientset()
	client.PrependReactor("list", "events", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, wantErr
	})
	if _, err := GetNamespaceEvents(client, dataselect.NoDataSelect, "default"); !errors.Is(err, wantErr) {
		t.Fatalf("GetNamespaceEvents() error = %v; want %v", err, wantErr)
	}
}

func TestGetEventListFromChannelsReturnsSecondChannelFailure(t *testing.T) {
	channels := &common.ResourceChannels{EventList: common.EventListChannel{
		List: make(chan *corev1.EventList, 1), Error: make(chan error, 2),
	}}
	channels.EventList.Error <- nil
	channels.EventList.List <- &corev1.EventList{}
	wantErr := apierrors.NewNotFound(schema.GroupResource{Resource: "events"}, "event")
	channels.EventList.Error <- wantErr
	if _, err := GetEventListFromChannels(channels, dataselect.NoDataSelect); !errors.Is(err, wantErr) {
		t.Fatalf("GetEventListFromChannels() error = %v; want %v", err, wantErr)
	}
}

func failingEventListClient() *fake.Clientset {
	client := fake.NewSimpleClientset()
	client.PrependReactor("list", "events", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("event list failed")
	})
	return client
}
