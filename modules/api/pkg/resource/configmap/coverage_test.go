// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package configmap

import (
	"errors"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"

	"k8s.io/dashboard/api/pkg/resource/common"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestConfigMapCellPropertiesAndConversions(t *testing.T) {
	created := metav1.NewTime(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	configMap := corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "settings", Namespace: "default", CreationTimestamp: created}}
	cell := ConfigMapCell(configMap)
	for _, tc := range []struct {
		property dataselect.PropertyName
		want     dataselect.ComparableValue
	}{
		{dataselect.NameProperty, dataselect.StdComparableString("settings")},
		{dataselect.NamespaceProperty, dataselect.StdComparableString("default")},
		{dataselect.CreationTimestampProperty, dataselect.StdComparableTime(created.Time)},
	} {
		if got := cell.GetProperty(tc.property); got != tc.want {
			t.Errorf("GetProperty(%s) = %v; want %v", tc.property, got, tc.want)
		}
	}
	if got := cell.GetProperty(dataselect.PropertyName("unsupported")); got != nil {
		t.Errorf("unsupported property = %v; want nil", got)
	}
	if got := fromCells(toCells([]corev1.ConfigMap{configMap})); !reflect.DeepEqual(got, []corev1.ConfigMap{configMap}) {
		t.Errorf("fromCells(toCells()) = %#v", got)
	}
}

func TestGetConfigMapListFromClient(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "default-map", Namespace: "default"}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "other-map", Namespace: "other"}},
	)
	got, err := GetConfigMapList(client, common.NewNamespaceQuery([]string{"default"}), dataselect.NoDataSelect)
	if err != nil {
		t.Fatalf("GetConfigMapList() error = %v", err)
	}
	if got.ListMeta.TotalItems != 1 || len(got.Items) != 1 || got.Items[0].ObjectMeta.Name != "default-map" {
		t.Fatalf("GetConfigMapList() = %#v; want only the default namespace config map", got)
	}
}

func TestGetConfigMapListFromChannelsErrors(t *testing.T) {
	client := fake.NewSimpleClientset()
	client.PrependReactor("list", "configmaps", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("list failed")
	})
	channels := &common.ResourceChannels{ConfigMapList: common.GetConfigMapListChannel(client, common.NewNamespaceQuery(nil), 1)}
	if _, err := GetConfigMapListFromChannels(channels, dataselect.NoDataSelect); err == nil || err.Error() != "list failed" {
		t.Fatalf("GetConfigMapListFromChannels() error = %v; want list failed", err)
	}
}
