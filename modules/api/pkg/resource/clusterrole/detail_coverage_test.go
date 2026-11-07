// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

package clusterrole

import (
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetClusterRoleDetailReturnsRulesAndPropagatesNotFound(t *testing.T) {
	rule := rbacv1.PolicyRule{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"watch"}}
	client := fake.NewSimpleClientset(&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "pod-watcher"}, Rules: []rbacv1.PolicyRule{rule}})
	got, err := GetClusterRoleDetail(client, "pod-watcher")
	if err != nil || got.ObjectMeta.Name != "pod-watcher" || len(got.Rules) != 1 || got.Rules[0].Verbs[0] != "watch" || len(got.Errors) != 0 {
		t.Fatalf("GetClusterRoleDetail() = %#v, %v", got, err)
	}
	if _, err := GetClusterRoleDetail(client, "missing"); err == nil {
		t.Error("GetClusterRoleDetail(missing) error = nil")
	}
}
