// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package event

import (
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"

	"k8s.io/dashboard/api/pkg/resource/common"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestEventCellPropertiesAndConversions(t *testing.T) {
	created := metav1.NewTime(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	event := corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "warning", Namespace: "default", CreationTimestamp: created}, Reason: "Failed", FirstTimestamp: created, LastTimestamp: created}
	cell := EventCell(event)
	for _, tc := range []struct {
		property dataselect.PropertyName
		want     dataselect.ComparableValue
	}{
		{dataselect.NameProperty, dataselect.StdComparableString("warning")},
		{dataselect.NamespaceProperty, dataselect.StdComparableString("default")},
		{dataselect.ReasonProperty, dataselect.StdComparableString("Failed")},
		{dataselect.CreationTimestampProperty, dataselect.StdComparableTime(created.Time)},
		{dataselect.FirstSeenProperty, dataselect.StdComparableTime(created.Time)},
		{dataselect.LastSeenProperty, dataselect.StdComparableTime(created.Time)},
	} {
		if got := cell.GetProperty(tc.property); got != tc.want {
			t.Errorf("GetProperty(%s) = %v; want %v", tc.property, got, tc.want)
		}
	}
	if cell.GetProperty(dataselect.PropertyName("unsupported")) != nil {
		t.Error("unsupported property should return nil")
	}
	if got := fromCells(toCells([]corev1.Event{event})); len(got) != 1 || got[0].Name != event.Name {
		t.Errorf("cell conversion = %#v", got)
	}
}

func TestClientBackedEventRetrieval(t *testing.T) {
	firstSeen := metav1.NewTime(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	client := fake.NewSimpleClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", UID: types.UID("pod-uid")}},
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-a", UID: types.UID("node-a")}},
		&corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "web-warning", Namespace: "default"}, Reason: "Failed", Message: "image pull failed", FirstTimestamp: firstSeen, InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "web", Namespace: "default", UID: "pod-uid"}},
		&corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "other-event", Namespace: "other"}, Reason: "Started", InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "other", Namespace: "other", UID: "other-uid"}},
		&corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "node-event", Namespace: "default"}, Reason: "NodeReady", InvolvedObject: corev1.ObjectReference{Kind: "Node", Name: "node-a", UID: "node-a"}},
	)

	got, err := GetEventList(client, common.NewSameNamespaceQuery("default"), dataselect.NoDataSelect)
	if err != nil || got.ListMeta.TotalItems != 2 || len(got.Events) != 2 {
		t.Fatalf("GetEventList() = %#v, %v", got, err)
	}
	resourceClient := fake.NewSimpleClientset(&corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "web-warning", Namespace: "default"}, Reason: "Failed", Message: "image pull failed", FirstTimestamp: firstSeen, InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "web", Namespace: "default", UID: "pod-uid"}})
	resourceEvents, err := GetResourceEvents(resourceClient, dataselect.NoDataSelect, "default", "web")
	if err != nil || resourceEvents.ListMeta.TotalItems != 1 || len(resourceEvents.Events) != 1 || resourceEvents.Events[0].Type != corev1.EventTypeWarning {
		t.Fatalf("GetResourceEvents() = %#v, %v", resourceEvents, err)
	}
	if got, err := GetPodsEvents(client, "default", []corev1.Pod{{ObjectMeta: metav1.ObjectMeta{UID: "pod-uid"}}}); err != nil || len(got) != 1 || got[0].Name != "web-warning" {
		t.Fatalf("GetPodsEvents() = %#v, %v", got, err)
	}
	if got, err := GetPodEvents(client, "default", "web"); err != nil || len(got) != 1 || got[0].Name != "web-warning" {
		t.Fatalf("GetPodEvents() = %#v, %v", got, err)
	}
	if got, err := GetNamespaceEvents(client, dataselect.NoDataSelect, "default"); err != nil || got.ListMeta.TotalItems != 2 {
		t.Fatalf("GetNamespaceEvents() = %#v, %v", got, err)
	}
	nodeEvents, err := GetNodeEvents(client, dataselect.NoDataSelect, "node-a")
	if err != nil || nodeEvents.ListMeta.TotalItems == 0 {
		t.Fatalf("GetNodeEvents() = %#v, %v", nodeEvents, err)
	}
}

func TestEventListAndResourceRetrievalErrors(t *testing.T) {
	client := fake.NewSimpleClientset()
	client.PrependReactor("list", "events", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("event list failed")
	})
	if _, err := GetEventList(client, common.NewNamespaceQuery(nil), dataselect.NoDataSelect); err == nil || err.Error() != "event list failed" {
		t.Fatalf("GetEventList() error = %v; want event list failed", err)
	}
	if _, err := GetResourceEvents(client, dataselect.NoDataSelect, "default", "web"); err == nil || err.Error() != "event list failed" {
		t.Fatalf("GetResourceEvents() error = %v; want event list failed", err)
	}
}
