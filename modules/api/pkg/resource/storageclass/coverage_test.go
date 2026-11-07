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

package storageclass

import (
	"testing"

	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestStorageClassCellPropertiesAndConversions(t *testing.T) {
	storageClass := storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "fast"}}
	cell := StorageClassCell(storageClass)
	if got := cell.GetProperty(dataselect.NameProperty); got != dataselect.StdComparableString("fast") {
		t.Errorf("name property = %v", got)
	}
	if got := cell.GetProperty(dataselect.CreationTimestampProperty); got == nil {
		t.Error("creation timestamp property is nil")
	}
	if got := cell.GetProperty(dataselect.NamespaceProperty); got != dataselect.StdComparableString("") {
		t.Errorf("namespace property = %v", got)
	}
	if got := cell.GetProperty("unsupported"); got != nil {
		t.Errorf("unsupported property = %v, want nil", got)
	}
	if got := fromCells(toCells([]storagev1.StorageClass{storageClass})); len(got) != 1 || got[0].Name != "fast" {
		t.Errorf("storage class cell conversion = %#v", got)
	}
}

func TestGetStorageClass(t *testing.T) {
	client := fake.NewClientset(&storagev1.StorageClass{
		ObjectMeta:  metav1.ObjectMeta{Name: "fast"},
		Provisioner: "example.test/provisioner",
		Parameters:  map[string]string{"type": "ssd"},
	})

	got, err := GetStorageClass(client, "fast")
	if err != nil {
		t.Fatalf("GetStorageClass() error = %v", err)
	}
	if got.ObjectMeta.Name != "fast" || got.Provisioner != "example.test/provisioner" || got.Parameters["type"] != "ssd" {
		t.Errorf("GetStorageClass() = %#v", got)
	}

	if _, err := GetStorageClass(client, "missing"); err == nil {
		t.Error("GetStorageClass() succeeded for a missing class")
	}
}
