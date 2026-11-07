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

package secret

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

func TestSecretCellPropertiesAndConversions(t *testing.T) {
	created := metav1.NewTime(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	secret := corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "credentials", Namespace: "default", CreationTimestamp: created}}
	cell := SecretCell(secret)
	for _, tc := range []struct {
		property dataselect.PropertyName
		want     dataselect.ComparableValue
	}{
		{dataselect.NameProperty, dataselect.StdComparableString("credentials")},
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
	if got := fromCells(toCells([]corev1.Secret{secret})); !reflect.DeepEqual(got, []corev1.Secret{secret}) {
		t.Errorf("fromCells(toCells()) = %#v", got)
	}
}

func TestImagePullSecretSpec(t *testing.T) {
	spec := &ImagePullSecretSpec{Name: "registry", Namespace: "default", Data: []byte("encoded")}
	if spec.GetName() != "registry" || spec.GetNamespace() != "default" || spec.GetType() != corev1.SecretTypeDockercfg {
		t.Fatalf("spec accessors returned unexpected values")
	}
	if got := spec.GetData(); !reflect.DeepEqual(got, map[string][]byte{corev1.DockerConfigKey: []byte("encoded")}) {
		t.Fatalf("GetData() = %#v", got)
	}
}

func TestCreateSecret(t *testing.T) {
	spec := &ImagePullSecretSpec{Name: "registry", Namespace: "default", Data: []byte("encoded")}
	client := fake.NewSimpleClientset()
	got, err := CreateSecret(client, spec)
	if err != nil {
		t.Fatalf("CreateSecret() error = %v", err)
	}
	created, err := client.CoreV1().Secrets("default").Get(t.Context(), "registry", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get created secret: %v", err)
	}
	if got.ObjectMeta.Name != "registry" || got.Type != corev1.SecretTypeDockercfg || string(created.Data[corev1.DockerConfigKey]) != "encoded" {
		t.Fatalf("created secret result = %#v, object = %#v", got, created)
	}

	client.PrependReactor("create", "secrets", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("create failed")
	})
	if _, err := CreateSecret(client, spec); err == nil || err.Error() != "create failed" {
		t.Fatalf("CreateSecret() error = %v; want create failed", err)
	}
}

func TestGetSecretListFromClient(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "default-secret", Namespace: "default"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "other-secret", Namespace: "other"}},
	)
	got, err := GetSecretList(client, common.NewNamespaceQuery([]string{"default"}), dataselect.NoDataSelect)
	if err != nil {
		t.Fatalf("GetSecretList() error = %v", err)
	}
	if got.TotalItems != 1 || len(got.Secrets) != 1 || got.Secrets[0].ObjectMeta.Name != "default-secret" {
		t.Fatalf("GetSecretList() = %#v; want only the default namespace secret", got)
	}
}

func TestGetSecretListReturnsClientError(t *testing.T) {
	client := fake.NewSimpleClientset()
	client.PrependReactor("list", "secrets", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("list failed")
	})
	if _, err := GetSecretList(client, common.NewNamespaceQuery(nil), dataselect.NoDataSelect); err == nil || err.Error() != "list failed" {
		t.Fatalf("GetSecretList() error = %v; want list failed", err)
	}
}
