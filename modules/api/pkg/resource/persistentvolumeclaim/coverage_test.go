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
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
package persistentvolumeclaim

import (
	"errors"
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

func TestPersistentVolumeClaimCellListAndDetail(t *testing.T) {
	created := metav1.NewTime(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	storageClass := "fast"
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "claim-a", Namespace: "default", CreationTimestamp: created}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "volume-a", StorageClassName: &storageClass}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}}
	cell := PersistentVolumeClaimCell(*pvc)
	for _, tc := range []struct {
		property dataselect.PropertyName
		want     dataselect.ComparableValue
	}{
		{dataselect.NameProperty, dataselect.StdComparableString("claim-a")},
		{dataselect.NamespaceProperty, dataselect.StdComparableString("default")},
		{dataselect.CreationTimestampProperty, dataselect.StdComparableTime(created.Time)},
	} {
		if got := cell.GetProperty(tc.property); got != tc.want {
			t.Errorf("GetProperty(%s) = %v; want %v", tc.property, got, tc.want)
		}
	}
	if cell.GetProperty(dataselect.PropertyName("unsupported")) != nil {
		t.Error("unsupported property should return nil")
	}
	if got := fromCells(toCells([]corev1.PersistentVolumeClaim{*pvc})); len(got) != 1 || got[0].Name != pvc.Name {
		t.Errorf("cell conversion = %#v", got)
	}
	client := fake.NewSimpleClientset(pvc)
	query := common.NewSameNamespaceQuery("default")
	list, err := GetPersistentVolumeClaimList(client, query, dataselect.NoDataSelect)
	if err != nil || list.ListMeta.TotalItems != 1 || list.Items[0].Volume != "volume-a" || list.Items[0].Status != string(corev1.ClaimBound) {
		t.Fatalf("GetPersistentVolumeClaimList() = %#v, %v", list, err)
	}
	detail, err := GetPersistentVolumeClaimDetail(client, "default", "claim-a")
	if err != nil || detail.ObjectMeta.Name != "claim-a" || detail.StorageClass == nil || *detail.StorageClass != storageClass {
		t.Fatalf("GetPersistentVolumeClaimDetail() = %#v, %v", detail, err)
	}
	if _, err := GetPersistentVolumeClaimDetail(client, "default", "missing"); err == nil {
		t.Fatal("GetPersistentVolumeClaimDetail() succeeded for missing claim")
	}
}

func TestPersistentVolumeClaimListReturnsClientError(t *testing.T) {
	client := fake.NewSimpleClientset()
	client.PrependReactor("list", "persistentvolumeclaims", func(ktesting.Action) (bool, runtime.Object, error) { return true, nil, errors.New("list failed") })
	if _, err := GetPersistentVolumeClaimList(client, common.NewNamespaceQuery(nil), dataselect.NoDataSelect); err == nil || err.Error() != "list failed" {
		t.Fatalf("GetPersistentVolumeClaimList() error = %v", err)
	}
}
