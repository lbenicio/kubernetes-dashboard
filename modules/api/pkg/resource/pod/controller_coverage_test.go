// Copyright 2026 The Kubernetes Authors.
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

package pod

import (
	"errors"
	"testing"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	"k8s.io/dashboard/api/pkg/resource/common"
	dashboardtypes "k8s.io/dashboard/types"
)

func TestGetPodControllerResolvesOwnerAndHandlesLookupFailures(t *testing.T) {
	t.Run("replica set owner", func(t *testing.T) {
		controlled := true
		pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Name: "web-0", Namespace: "default", UID: types.UID("pod-uid"),
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "web", UID: types.UID("rs-uid"), Controller: &controlled}},
		}}
		replicaSet := &apps.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", UID: types.UID("rs-uid"), Labels: map[string]string{"app": "web"}},
			Spec:       apps.ReplicaSetSpec{Replicas: int32Pointer(1), Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}}},
		}
		client := fake.NewSimpleClientset(pod, replicaSet)
		owner, err := getPodController(client, common.NewSameNamespaceQuery("default"), pod)
		if err != nil {
			t.Fatalf("getPodController() error = %v", err)
		}
		if owner.TypeMeta.Kind != dashboardtypes.ResourceKindReplicaSet || owner.ObjectMeta.Name != "web" {
			t.Errorf("resolved owner = %#v; want ReplicaSet web", owner)
		}
	})

	t.Run("no controller owner", func(t *testing.T) {
		client := fake.NewSimpleClientset()
		owner, err := getPodController(client, common.NewSameNamespaceQuery("default"), &corev1.Pod{})
		if err != nil || owner == nil || owner.TypeMeta.Kind != "" {
			t.Fatalf("getPodController(no owner) = %#v, %v; want empty owner", owner, err)
		}
	})

	t.Run("unknown owner kind is ignored", func(t *testing.T) {
		controlled := true
		pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{Kind: "Unknown", Name: "owner", Controller: &controlled}}}}
		owner, err := getPodController(fake.NewSimpleClientset(), common.NewSameNamespaceQuery("default"), pod)
		if err != nil || owner == nil || owner.TypeMeta.Kind != "" {
			t.Fatalf("getPodController(unknown owner) = %#v, %v; want empty owner and no error", owner, err)
		}
	})

	t.Run("pod list error is returned", func(t *testing.T) {
		client := fake.NewSimpleClientset()
		client.PrependReactor("list", "pods", func(clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("pod list failed")
		})
		if _, err := getPodController(client, common.NewSameNamespaceQuery("default"), &corev1.Pod{}); err == nil {
			t.Fatal("getPodController(pod list error) error = nil")
		}
	})

	t.Run("event list error is ignored", func(t *testing.T) {
		client := fake.NewSimpleClientset()
		client.PrependReactor("list", "events", func(clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("event list failed")
		})
		owner, err := getPodController(client, common.NewSameNamespaceQuery("default"), &corev1.Pod{})
		if err != nil || owner == nil {
			t.Fatalf("getPodController(event list error) = %#v, %v; want non-fatal empty owner", owner, err)
		}
	})
}

func int32Pointer(value int32) *int32 { return &value }
