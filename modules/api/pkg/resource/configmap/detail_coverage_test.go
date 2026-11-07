// Copyright 2026 The Kubernetes Authors.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
package configmap

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetConfigMapDetailClient(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "settings", Namespace: "default"}, Data: map[string]string{"mode": "safe"}})
	detail, err := GetConfigMapDetail(client, "default", "settings")
	if err != nil || detail.ObjectMeta.Name != "settings" || detail.Data["mode"] != "safe" {
		t.Fatalf("GetConfigMapDetail() = %#v, %v", detail, err)
	}
	if _, err := GetConfigMapDetail(client, "default", "missing"); err == nil {
		t.Fatal("GetConfigMapDetail() succeeded for missing config map")
	}
}
