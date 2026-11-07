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

package replicaset

import (
	"testing"
	"time"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	"k8s.io/dashboard/api/pkg/resource/dataselect"
	dashboardtypes "k8s.io/dashboard/types"
)

func TestReplicaSetCellPropertiesAndSelector(t *testing.T) {
	created := metav1.NewTime(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	rs := apps.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", UID: types.UID("uid"), CreationTimestamp: created}}
	cell := ReplicaSetCell(rs)
	for _, tc := range []struct {
		property dataselect.PropertyName
		want     dataselect.ComparableValue
	}{
		{dataselect.NameProperty, dataselect.StdComparableString("web")},
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
	selector := cell.GetResourceSelector()
	if selector.Namespace != "default" || selector.ResourceType != dashboardtypes.ResourceKindReplicaSet || selector.ResourceName != "web" || selector.UID != "uid" {
		t.Errorf("GetResourceSelector() = %#v", selector)
	}
	if got := FromCells(ToCells([]apps.ReplicaSet{rs})); len(got) != 1 || got[0].Name != rs.Name {
		t.Errorf("cell conversion = %#v", got)
	}
}

func TestReplicaSetClientBackedPodsAndServices(t *testing.T) {
	controlled := true
	count := int32(1)
	rs := &apps.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", UID: types.UID("rs-uid")}, Spec: apps.ReplicaSetSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}}, Replicas: &count}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web-pod", Namespace: "default", Labels: map[string]string{"app": "web"}, OwnerReferences: []metav1.OwnerReference{{Kind: "ReplicaSet", Name: "web", UID: rs.UID, Controller: &controlled}}}}
	service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "web-service", Namespace: "default"}, Spec: corev1.ServiceSpec{Selector: map[string]string{"app": "web"}}}
	client := fake.NewSimpleClientset(rs, pod, service)
	pods, err := GetReplicaSetPods(client, nil, dataselect.NoDataSelect, "web", "default")
	if err != nil || len(pods.Pods) != 1 || pods.Pods[0].ObjectMeta.Name != "web-pod" {
		t.Fatalf("GetReplicaSetPods() = %#v, %v", pods, err)
	}
	services, err := GetReplicaSetServices(client, dataselect.NoDataSelect, "default", "web")
	if err != nil || services.ListMeta.TotalItems != 1 || services.Services[0].ObjectMeta.Name != "web-service" {
		t.Fatalf("GetReplicaSetServices() = %#v, %v", services, err)
	}
	if _, err := GetReplicaSetPods(client, nil, dataselect.NoDataSelect, "missing", "default"); err == nil {
		t.Fatal("GetReplicaSetPods() succeeded for missing replica set")
	}
	if _, err := GetReplicaSetServices(client, dataselect.NoDataSelect, "default", "missing"); err == nil {
		t.Fatal("GetReplicaSetServices() succeeded for missing replica set")
	}
}
