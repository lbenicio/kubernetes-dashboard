// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package statefulset

import (
	"testing"
	"time"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	"k8s.io/dashboard/api/pkg/resource/common"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestStatefulSetCellPropertiesAndSelector(t *testing.T) {
	created := metav1.NewTime(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	statefulSet := apps.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "database", Namespace: "default", UID: types.UID("uid"), CreationTimestamp: created}}
	cell := StatefulSetCell(statefulSet)
	for _, tc := range []struct {
		property dataselect.PropertyName
		want     dataselect.ComparableValue
	}{
		{dataselect.NameProperty, dataselect.StdComparableString("database")},
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
	if selector.Namespace != "default" || selector.ResourceType != "statefulset" || selector.ResourceName != "database" || selector.UID != "uid" {
		t.Errorf("GetResourceSelector() = %#v", selector)
	}
	if got := fromCells(toCells([]apps.StatefulSet{statefulSet})); len(got) != 1 || got[0].Name != statefulSet.Name {
		t.Errorf("cell conversion = %#v", got)
	}
}

func TestStatefulSetClientBackedListDetailsPodsAndRestart(t *testing.T) {
	controlled := true
	replicas := int32(2)
	statefulSet := &apps.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "database", Namespace: "default", UID: types.UID("statefulset-uid")},
		Spec: apps.StatefulSetSpec{
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
				Containers:     []corev1.Container{{Name: "db", Image: "postgres:17"}},
				InitContainers: []corev1.Container{{Name: "init", Image: "busybox:1"}},
			}},
		},
		Status: apps.StatefulSetStatus{Replicas: 1},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "database-0", Namespace: "default", OwnerReferences: []metav1.OwnerReference{{Kind: "StatefulSet", Name: statefulSet.Name, UID: statefulSet.UID, Controller: &controlled}}},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	client := fake.NewSimpleClientset(statefulSet, pod)

	list, err := GetStatefulSetList(client, common.NewSameNamespaceQuery("default"), dataselect.NoDataSelect, nil)
	if err != nil || list.ListMeta.TotalItems != 1 || len(list.StatefulSets) != 1 {
		t.Fatalf("GetStatefulSetList() = %#v, %v", list, err)
	}
	if list.StatefulSets[0].ContainerImages[0] != "postgres:17" || list.StatefulSets[0].InitContainerImages[0] != "busybox:1" {
		t.Errorf("list container images = %#v / %#v", list.StatefulSets[0].ContainerImages, list.StatefulSets[0].InitContainerImages)
	}

	detail, err := GetStatefulSetDetail(client, nil, "default", "database")
	if err != nil || detail.ObjectMeta.Name != "database" || detail.Pods.Current != 1 || detail.Pods.Desired == nil || *detail.Pods.Desired != 2 {
		t.Fatalf("GetStatefulSetDetail() = %#v, %v", detail, err)
	}

	pods, err := GetStatefulSetPods(client, nil, dataselect.NoDataSelect, "database", "default")
	if err != nil || len(pods.Pods) != 1 || pods.Pods[0].ObjectMeta.Name != "database-0" {
		t.Fatalf("GetStatefulSetPods() = %#v, %v", pods, err)
	}

	restarted, err := RestartStatefulSet(client, "default", "database")
	if err != nil || restarted.Spec.Template.Annotations[RestartedAtAnnotationKey] == "" {
		t.Fatalf("RestartStatefulSet() = %#v, %v", restarted, err)
	}
	if _, err := GetStatefulSetDetail(client, nil, "default", "missing"); err == nil {
		t.Fatal("GetStatefulSetDetail() succeeded for missing stateful set")
	}
	if _, err := RestartStatefulSet(client, "default", "missing"); err == nil {
		t.Fatal("RestartStatefulSet() succeeded for missing stateful set")
	}
}
