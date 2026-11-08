// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package replicaset

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/dashboard/api/pkg/resource/common"
)

func TestGetStatusClassifiesReplicaSets(t *testing.T) {
	if got := getStatus(nil, nil, nil); got != (common.ResourceStatus{}) {
		t.Fatalf("getStatus(nil) = %#v; want empty status", got)
	}

	sets := []appsv1.ReplicaSet{
		{ObjectMeta: metav1.ObjectMeta{Name: "failed", UID: "rs-failed"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "pending", UID: "rs-pending"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "running", UID: "rs-running"}},
	}
	pods := []corev1.Pod{
		ownedReplicaSetPod("pod-failed", "rs-failed", corev1.PodFailed),
		ownedReplicaSetPod("pod-pending", "rs-pending", corev1.PodPending),
		ownedReplicaSetPod("pod-running", "rs-running", corev1.PodRunning),
	}
	events := []corev1.Event{{
		Type:           corev1.EventTypeWarning,
		Reason:         "FailedScheduling",
		InvolvedObject: corev1.ObjectReference{UID: "pod-failed"},
	}}

	got := getStatus(&appsv1.ReplicaSetList{Items: sets}, pods, events)
	want := common.ResourceStatus{Failed: 1, Pending: 1, Running: 1}
	if got != want {
		t.Fatalf("getStatus() = %#v; want %#v", got, want)
	}
}

func ownedReplicaSetPod(name, ownerUID string, phase corev1.PodPhase) corev1.Pod {
	controller := true
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			UID:  types.UID(name),
			OwnerReferences: []metav1.OwnerReference{{
				UID: types.UID(ownerUID), Controller: &controller,
			}},
		},
		Status: corev1.PodStatus{Phase: phase},
	}
}
