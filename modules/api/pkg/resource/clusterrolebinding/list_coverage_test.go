// Copyright 2026 The Kubernetes Authors.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
package clusterrolebinding

import (
	"errors"
	"testing"
	"time"

	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestGetClusterRoleBindingListAndCell(t *testing.T) {
	created := metav1.NewTime(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	client := fake.NewSimpleClientset(&rbac.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "readers", CreationTimestamp: created}})
	got, err := GetClusterRoleBindingList(client, dataselect.NoDataSelect)
	if err != nil || got.ListMeta.TotalItems != 1 || got.Items[0].ObjectMeta.Name != "readers" {
		t.Fatalf("GetClusterRoleBindingList() = %#v, %v", got, err)
	}
	cell := ClusterRoleBindingCell(got.Items[0])
	if cell.GetProperty(dataselect.NameProperty) != dataselect.StdComparableString("readers") || cell.GetProperty(dataselect.CreationTimestampProperty) != dataselect.StdComparableTime(created.Time) || cell.GetProperty(dataselect.NamespaceProperty) != dataselect.StdComparableString("") || cell.GetProperty(dataselect.PropertyName("unknown")) != nil {
		t.Fatal("ClusterRoleBindingCell.GetProperty() returned unexpected values")
	}
	if len(fromCells(toCells(got.Items))) != 1 {
		t.Fatal("cell conversion did not preserve the item")
	}
}

func TestGetClusterRoleBindingListReturnsClientError(t *testing.T) {
	client := fake.NewSimpleClientset()
	client.PrependReactor("list", "clusterrolebindings", func(ktesting.Action) (bool, runtime.Object, error) { return true, nil, errors.New("list failed") })
	if _, err := GetClusterRoleBindingList(client, dataselect.NoDataSelect); err == nil || err.Error() != "list failed" {
		t.Fatalf("GetClusterRoleBindingList() error = %v", err)
	}
}
