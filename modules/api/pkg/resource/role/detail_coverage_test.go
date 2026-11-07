// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

package role

import (
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetRoleDetailReturnsRulesAndPropagatesNotFound(t *testing.T) {
	rule := rbacv1.PolicyRule{APIGroups: []string{"apps"}, Resources: []string{"deployments"}, Verbs: []string{"get", "list"}}
	client := fake.NewSimpleClientset(&rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "reader", Namespace: "default"}, Rules: []rbacv1.PolicyRule{rule}})
	got, err := GetRoleDetail(client, "default", "reader")
	if err != nil || got.ObjectMeta.Name != "reader" || len(got.Rules) != 1 || got.Rules[0].Verbs[1] != "list" || len(got.Errors) != 0 {
		t.Fatalf("GetRoleDetail() = %#v, %v", got, err)
	}
	if _, err := GetRoleDetail(client, "default", "missing"); err == nil {
		t.Error("GetRoleDetail(missing) error = nil")
	}
}
