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
package persistentvolume

import (
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestPersistentVolumeCellAndClientList(t *testing.T) {
	created := metav1.NewTime(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: "volume-a", CreationTimestamp: created}, Spec: corev1.PersistentVolumeSpec{StorageClassName: "fast"}, Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeAvailable}}
	cell := PersistentVolumeCell(*pv)
	for _, tc := range []struct {
		property dataselect.PropertyName
		want     dataselect.ComparableValue
	}{
		{dataselect.NameProperty, dataselect.StdComparableString("volume-a")},
		{dataselect.NamespaceProperty, dataselect.StdComparableString("")},
		{dataselect.CreationTimestampProperty, dataselect.StdComparableTime(created.Time)},
		{dataselect.StatusProperty, dataselect.StdComparableString(string(corev1.VolumeAvailable))},
	} {
		if got := cell.GetProperty(tc.property); got != tc.want {
			t.Errorf("GetProperty(%s) = %v; want %v", tc.property, got, tc.want)
		}
	}
	if cell.GetProperty(dataselect.PropertyName("unsupported")) != nil {
		t.Error("unsupported property should return nil")
	}
	if got := fromCells(toCells([]corev1.PersistentVolume{*pv})); len(got) != 1 || got[0].Name != pv.Name {
		t.Errorf("cell conversion = %#v", got)
	}
	list, err := GetPersistentVolumeList(fake.NewSimpleClientset(pv), dataselect.NoDataSelect)
	if err != nil || list.ListMeta.TotalItems != 1 || list.Items[0].StorageClass != "fast" || list.Items[0].Status != corev1.VolumeAvailable {
		t.Fatalf("GetPersistentVolumeList() = %#v, %v", list, err)
	}
}

func TestPersistentVolumeListReturnsClientError(t *testing.T) {
	client := fake.NewSimpleClientset()
	client.PrependReactor("list", "persistentvolumes", func(ktesting.Action) (bool, runtime.Object, error) { return true, nil, errors.New("list failed") })
	if _, err := GetPersistentVolumeList(client, dataselect.NoDataSelect); err == nil || err.Error() != "list failed" {
		t.Fatalf("GetPersistentVolumeList() error = %v", err)
	}
}
