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

package serviceaccount

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"k8s.io/dashboard/api/pkg/resource/common"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestServiceAccountListAndDetail(t *testing.T) {
	account := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "dashboard", Namespace: "team-a"}}
	client := fake.NewSimpleClientset(account)
	list, err := GetServiceAccountList(client, common.NewSameNamespaceQuery("team-a"), dataselect.NoDataSelect)
	if err != nil || len(list.Items) != 1 || list.Items[0].Name != "dashboard" {
		t.Fatalf("GetServiceAccountList() = %#v, %v", list, err)
	}
	detail, err := GetServiceAccountDetail(client, "team-a", "dashboard")
	if err != nil || detail.Name != "dashboard" {
		t.Fatalf("GetServiceAccountDetail() = %#v, %v", detail, err)
	}
	if _, err := GetServiceAccountDetail(client, "team-a", "missing"); err == nil {
		t.Fatal("GetServiceAccountDetail() accepted a missing account")
	}
}

func TestServiceAccountListsReferencedSecrets(t *testing.T) {
	account := &corev1.ServiceAccount{
		ObjectMeta:       metav1.ObjectMeta{Name: "dashboard", Namespace: "team-a"},
		ImagePullSecrets: []corev1.LocalObjectReference{{Name: "registry"}},
		Secrets:          []corev1.ObjectReference{{Name: "token"}},
	}
	client := fake.NewSimpleClientset(
		account,
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "registry", Namespace: "team-a"}, Type: corev1.SecretTypeDockerConfigJson},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "token", Namespace: "team-a"}, Type: corev1.SecretTypeServiceAccountToken},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "unreferenced", Namespace: "team-a"}},
	)

	imagePullSecrets, err := GetServiceAccountImagePullSecrets(client, "team-a", "dashboard", dataselect.NoDataSelect)
	if err != nil || len(imagePullSecrets.Secrets) != 1 || imagePullSecrets.Secrets[0].ObjectMeta.Name != "registry" {
		t.Fatalf("GetServiceAccountImagePullSecrets() = %#v, %v", imagePullSecrets, err)
	}
	secrets, err := GetServiceAccountSecrets(client, "team-a", "dashboard", dataselect.NoDataSelect)
	if err != nil || len(secrets.Secrets) != 1 || secrets.Secrets[0].ObjectMeta.Name != "token" {
		t.Fatalf("GetServiceAccountSecrets() = %#v, %v", secrets, err)
	}
}

func TestServiceAccountSecretListsHandleEmptyAndMissingReferences(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "empty", Namespace: "team-a"}})
	imagePullSecrets, err := GetServiceAccountImagePullSecrets(client, "team-a", "empty", dataselect.NoDataSelect)
	if err != nil || len(imagePullSecrets.Secrets) != 0 {
		t.Fatalf("empty image pull secrets = %#v, %v", imagePullSecrets, err)
	}
	secrets, err := GetServiceAccountSecrets(client, "team-a", "empty", dataselect.NoDataSelect)
	if err != nil || len(secrets.Secrets) != 0 {
		t.Fatalf("empty secrets = %#v, %v", secrets, err)
	}
	if _, err := GetServiceAccountSecrets(client, "team-a", "missing", dataselect.NoDataSelect); err == nil {
		t.Fatal("GetServiceAccountSecrets() accepted a missing account")
	}
	if _, err := GetServiceAccountImagePullSecrets(client, "team-a", "missing", dataselect.NoDataSelect); err == nil {
		t.Fatal("GetServiceAccountImagePullSecrets() accepted a missing account")
	}
}

func TestServiceAccountDataCells(t *testing.T) {
	cell := ServiceAccountCell(corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "account", Namespace: "team-a"}})
	if got := cell.GetProperty(dataselect.NameProperty); got != dataselect.StdComparableString("account") {
		t.Errorf("name property = %v", got)
	}
	if got := cell.GetProperty(dataselect.NamespaceProperty); got != dataselect.StdComparableString("team-a") {
		t.Errorf("namespace property = %v", got)
	}
	if got := cell.GetProperty(dataselect.CreationTimestampProperty); got == nil {
		t.Error("creation timestamp property is nil")
	}
	if got := cell.GetProperty("other"); got != nil {
		t.Errorf("unsupported property = %v", got)
	}
	if got := fromCells(toCells([]corev1.ServiceAccount{{ObjectMeta: metav1.ObjectMeta{Name: "account"}}})); len(got) != 1 || got[0].Name != "account" {
		t.Errorf("service account cell conversion = %#v", got)
	}
}
