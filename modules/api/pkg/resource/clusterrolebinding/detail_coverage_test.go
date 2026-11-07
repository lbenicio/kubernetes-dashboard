// Copyright 2017 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

package clusterrolebinding

import (
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetClusterRoleBindingDetailReturnsSubjectsAndReference(t *testing.T) {
	binding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-readers"},
		Subjects:   []rbacv1.Subject{{Kind: "Group", Name: "operators"}},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "reader"},
	}
	client := fake.NewSimpleClientset(binding)
	got, err := GetClusterRoleBindingDetail(client, "cluster-readers")
	if err != nil || len(got.Subjects) != 1 || got.Subjects[0].Name != "operators" || got.RoleRef.Name != "reader" || len(got.Errors) != 0 {
		t.Fatalf("GetClusterRoleBindingDetail() = %#v, %v", got, err)
	}
	if _, err := GetClusterRoleBindingDetail(client, "missing"); err == nil {
		t.Error("GetClusterRoleBindingDetail(missing) error = nil")
	}
}
