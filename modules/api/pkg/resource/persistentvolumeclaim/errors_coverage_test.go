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

package persistentvolumeclaim

import (
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestGetPodPersistentVolumeClaimsHandlesMissingAndFailedLookups(t *testing.T) {
	query := dataselect.NoDataSelect
	wantErr := errors.New("persistent volume claim lookup failed")

	t.Run("pod get failure", func(t *testing.T) {
		client := fake.NewSimpleClientset()
		client.PrependReactor("get", "pods", func(ktesting.Action) (bool, runtime.Object, error) {
			return true, nil, wantErr
		})
		if _, err := GetPodPersistentVolumeClaims(client, "default", "app", query); !errors.Is(err, wantErr) {
			t.Fatalf("GetPodPersistentVolumeClaims() error = %v; want %v", err, wantErr)
		}
	})

	t.Run("no claim volumes", func(t *testing.T) {
		pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"}, Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{{Name: "config", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}},
		}}
		got, err := GetPodPersistentVolumeClaims(fake.NewSimpleClientset(pod), pod.Namespace, pod.Name, query)
		if err != nil || got == nil || got.ListMeta.TotalItems != 0 || len(got.Items) != 0 {
			t.Fatalf("GetPodPersistentVolumeClaims() = %#v, %v; want empty list", got, err)
		}
	})

	t.Run("claim list failure", func(t *testing.T) {
		pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"}, Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data"}}}},
		}}
		client := fake.NewSimpleClientset(pod)
		client.PrependReactor("list", "persistentvolumeclaims", func(ktesting.Action) (bool, runtime.Object, error) {
			return true, nil, wantErr
		})
		if _, err := GetPodPersistentVolumeClaims(client, pod.Namespace, pod.Name, query); !errors.Is(err, wantErr) {
			t.Fatalf("GetPodPersistentVolumeClaims() error = %v; want %v", err, wantErr)
		}
	})
}
