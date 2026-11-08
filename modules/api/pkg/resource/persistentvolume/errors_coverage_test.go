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

package persistentvolume

import (
	"errors"
	"testing"

	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestGetStorageClassPersistentVolumesPropagatesClientErrors(t *testing.T) {
	wantErr := errors.New("storage request failed")

	t.Run("storage class get", func(t *testing.T) {
		client := fake.NewSimpleClientset()
		client.PrependReactor("get", "storageclasses", func(ktesting.Action) (bool, runtime.Object, error) {
			return true, nil, wantErr
		})
		if _, err := GetStorageClassPersistentVolumes(client, "fast", dataselect.NoDataSelect); !errors.Is(err, wantErr) {
			t.Fatalf("GetStorageClassPersistentVolumes() error = %v; want %v", err, wantErr)
		}
	})

	t.Run("persistent volume list", func(t *testing.T) {
		storageClass := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "fast"}}
		client := fake.NewSimpleClientset(storageClass)
		client.PrependReactor("list", "persistentvolumes", func(ktesting.Action) (bool, runtime.Object, error) {
			return true, nil, wantErr
		})
		if _, err := GetStorageClassPersistentVolumes(client, storageClass.Name, dataselect.NoDataSelect); !errors.Is(err, wantErr) {
			t.Fatalf("GetStorageClassPersistentVolumes() error = %v; want %v", err, wantErr)
		}
	})
}
