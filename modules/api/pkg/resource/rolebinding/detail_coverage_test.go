// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

package rolebinding

import (
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetRoleBindingDetailReturnsSubjectsAndReference(t *testing.T) {
	binding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "readers", Namespace: "default"},
		Subjects:   []rbacv1.Subject{{Kind: "User", Name: "alice"}},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: "reader"},
	}
	client := fake.NewSimpleClientset(binding)
	got, err := GetRoleBindingDetail(client, "default", "readers")
	if err != nil || len(got.Subjects) != 1 || got.Subjects[0].Name != "alice" || got.RoleRef.Name != "reader" || len(got.Errors) != 0 {
		t.Fatalf("GetRoleBindingDetail() = %#v, %v", got, err)
	}
	if _, err := GetRoleBindingDetail(client, "default", "missing"); err == nil {
		t.Error("GetRoleBindingDetail(missing) error = nil")
	}
}
