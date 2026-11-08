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

package statefulset

import (
	"errors"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestGetStatefulSetPodsReturnsClientFailures(t *testing.T) {
	statefulSet := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "database", Namespace: "default"}}

	t.Run("stateful set lookup error", func(t *testing.T) {
		client := fake.NewSimpleClientset()
		client.PrependReactor("get", "statefulsets", func(clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("stateful set lookup failed")
		})
		if _, err := GetStatefulSetPods(client, nil, dataselect.NoDataSelect, statefulSet.Name, statefulSet.Namespace); err == nil {
			t.Fatal("GetStatefulSetPods() error = nil; want lookup error")
		}
	})

	t.Run("pod list error", func(t *testing.T) {
		client := fake.NewSimpleClientset(statefulSet)
		client.PrependReactor("list", "pods", func(clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("pod list failed")
		})
		if _, err := GetStatefulSetPods(client, nil, dataselect.NoDataSelect, statefulSet.Name, statefulSet.Namespace); err == nil {
			t.Fatal("GetStatefulSetPods() error = nil; want pod list error")
		}
	})

	t.Run("event list error", func(t *testing.T) {
		client := fake.NewSimpleClientset(statefulSet, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "database-0", Namespace: "default"}})
		client.PrependReactor("list", "events", func(clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("event list failed")
		})
		if _, err := GetStatefulSetPods(client, nil, dataselect.NoDataSelect, statefulSet.Name, statefulSet.Namespace); err == nil {
			t.Fatal("GetStatefulSetPods() error = nil; want event list error")
		}
	})
}

func TestGetStatefulSetDetailReturnsPodListFailure(t *testing.T) {
	statefulSet := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "database", Namespace: "default"}}
	client := fake.NewSimpleClientset(statefulSet)
	client.PrependReactor("list", "pods", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("pod list failed")
	})
	if _, err := GetStatefulSetDetail(client, nil, statefulSet.Namespace, statefulSet.Name); err == nil {
		t.Fatal("GetStatefulSetDetail() error = nil; want pod list failure")
	}
}
