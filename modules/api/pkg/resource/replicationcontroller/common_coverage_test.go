// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package replicationcontroller

import (
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	"k8s.io/dashboard/api/pkg/resource/common"
)

func TestReplicationControllerHelpersHandleInvalidSelectorsAndListFailures(t *testing.T) {
	if _, err := toLabelSelector(map[string]string{"invalid key": "value"}); err == nil {
		t.Fatal("toLabelSelector() accepted an invalid label key")
	}

	selector, err := toLabelSelector(map[string]string{"app": "web"})
	if err != nil {
		t.Fatal(err)
	}
	t.Run("controller list failure", func(t *testing.T) {
		client := fake.NewSimpleClientset()
		client.PrependReactor("list", "replicationcontrollers", func(clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("controller list failed")
		})
		if _, err := getServicesForDeletion(client, selector, "default"); err == nil {
			t.Fatal("getServicesForDeletion() succeeded after listing controllers failed")
		}
	})

	t.Run("service list failure", func(t *testing.T) {
		controller := &corev1.ReplicationController{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Labels: map[string]string{"app": "web"}}, Spec: corev1.ReplicationControllerSpec{Selector: map[string]string{"app": "web"}}}
		client := fake.NewSimpleClientset(controller)
		client.PrependReactor("list", "services", func(clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("service list failed")
		})
		if _, err := getServicesForDeletion(client, selector, "default"); err == nil {
			t.Fatal("getServicesForDeletion() succeeded after listing services failed")
		}
	})
}

func TestReplicationControllerStatusClassifiesWarningsPendingAndRunning(t *testing.T) {
	if got := getStatus(nil, nil, nil); got != (common.ResourceStatus{}) {
		t.Fatalf("getStatus(nil) = %#v", got)
	}

	replicas := int32(1)
	controller := corev1.ReplicationController{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", UID: "controller-uid"},
		Spec:       corev1.ReplicationControllerSpec{Replicas: &replicas},
	}
	owner := metav1.OwnerReference{APIVersion: "v1", Kind: "ReplicationController", Name: "web", UID: controller.UID, Controller: boolPtr(true)}
	for _, test := range []struct {
		name   string
		pod    corev1.Pod
		events []corev1.Event
		want   common.ResourceStatus
	}{
		{
			name:   "warning event",
			pod:    corev1.Pod{ObjectMeta: metav1.ObjectMeta{UID: "pod-uid", OwnerReferences: []metav1.OwnerReference{owner}}, Status: corev1.PodStatus{Phase: corev1.PodFailed}},
			events: []corev1.Event{{Type: corev1.EventTypeWarning, Reason: "Failed", InvolvedObject: corev1.ObjectReference{UID: "pod-uid"}}},
			want:   common.ResourceStatus{Failed: 1},
		},
		{
			name: "pending pod",
			pod:  corev1.Pod{ObjectMeta: metav1.ObjectMeta{UID: "pod-uid", OwnerReferences: []metav1.OwnerReference{owner}}, Status: corev1.PodStatus{Phase: corev1.PodPending}},
			want: common.ResourceStatus{Pending: 1},
		},
		{
			name: "running pod",
			pod:  corev1.Pod{ObjectMeta: metav1.ObjectMeta{UID: "pod-uid", OwnerReferences: []metav1.OwnerReference{owner}}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
			want: common.ResourceStatus{Running: 1},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := getStatus(&corev1.ReplicationControllerList{Items: []corev1.ReplicationController{controller}}, []corev1.Pod{test.pod}, test.events)
			if got != test.want {
				t.Errorf("getStatus() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func boolPtr(value bool) *bool { return &value }
