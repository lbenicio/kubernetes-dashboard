// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package replicationcontroller

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	"k8s.io/dashboard/api/pkg/resource/dataselect"
	dashboardtypes "k8s.io/dashboard/types"
)

func TestReplicationControllerCellPropertiesAndSelector(t *testing.T) {
	created := metav1.NewTime(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	rc := corev1.ReplicationController{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", UID: types.UID("uid"), CreationTimestamp: created}}
	cell := ReplicationControllerCell(rc)
	for _, tc := range []struct {
		property dataselect.PropertyName
		want     dataselect.ComparableValue
	}{
		{dataselect.NameProperty, dataselect.StdComparableString("web")},
		{dataselect.NamespaceProperty, dataselect.StdComparableString("default")},
		{dataselect.CreationTimestampProperty, dataselect.StdComparableTime(created.Time)},
	} {
		if got := cell.GetProperty(tc.property); got != tc.want {
			t.Errorf("GetProperty(%s) = %v; want %v", tc.property, got, tc.want)
		}
	}
	if cell.GetProperty(dataselect.PropertyName("unsupported")) != nil {
		t.Error("unsupported property should return nil")
	}
	selector := cell.GetResourceSelector()
	if selector.Namespace != "default" || selector.ResourceType != dashboardtypes.ResourceKindReplicationController || selector.ResourceName != "web" || selector.UID != "uid" {
		t.Errorf("GetResourceSelector() = %#v", selector)
	}
	if got := fromCells(toCells([]corev1.ReplicationController{rc})); len(got) != 1 || got[0].Name != rc.Name {
		t.Errorf("cell conversion = %#v", got)
	}
}

func TestReplicationControllerClientBackedDetailPodsAndServices(t *testing.T) {
	controlled := true
	replicas := int32(1)
	rc := &corev1.ReplicationController{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", UID: types.UID("rc-uid")},
		Spec:       corev1.ReplicationControllerSpec{Replicas: &replicas, Selector: map[string]string{"app": "web"}, Template: &corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "web"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "web", Image: "nginx:1"}}}}},
		Status:     corev1.ReplicationControllerStatus{Replicas: 1},
	}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web-pod", Namespace: "default", UID: types.UID("pod-uid"), Labels: map[string]string{"app": "web"}, OwnerReferences: []metav1.OwnerReference{{Kind: "ReplicationController", Name: rc.Name, UID: rc.UID, Controller: &controlled}}}}
	service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "web-service", Namespace: "default"}, Spec: corev1.ServiceSpec{Selector: map[string]string{"app": "web"}}}
	client := fake.NewSimpleClientset(rc, pod, service)
	detail, err := GetReplicationControllerDetail(client, "default", "web")
	if err != nil || detail.ObjectMeta.Name != "web" || detail.Pods.Current != 1 {
		t.Fatalf("GetReplicationControllerDetail() = %#v, %v", detail, err)
	}
	pods, err := GetReplicationControllerPods(client, nil, dataselect.NoDataSelect, "web", "default")
	if err != nil || len(pods.Pods) != 1 || pods.Pods[0].ObjectMeta.Name != "web-pod" {
		t.Fatalf("GetReplicationControllerPods() = %#v, %v", pods, err)
	}
	services, err := GetReplicationControllerServices(client, dataselect.NoDataSelect, "default", "web")
	if err != nil || services.ListMeta.TotalItems != 1 || services.Services[0].ObjectMeta.Name != "web-service" {
		t.Fatalf("GetReplicationControllerServices() = %#v, %v", services, err)
	}
	if _, err := GetReplicationControllerDetail(client, "default", "missing"); err == nil {
		t.Fatal("GetReplicationControllerDetail() succeeded for missing controller")
	}
	if _, err := GetReplicationControllerPods(client, nil, dataselect.NoDataSelect, "missing", "default"); err == nil {
		t.Fatal("GetReplicationControllerPods() succeeded for missing controller")
	}
	if _, err := GetReplicationControllerServices(client, dataselect.NoDataSelect, "default", "missing"); err == nil {
		t.Fatal("GetReplicationControllerServices() succeeded for missing controller")
	}
}
