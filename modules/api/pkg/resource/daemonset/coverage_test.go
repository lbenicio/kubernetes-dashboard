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

package daemonset

import (
	"testing"
	"time"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	"k8s.io/dashboard/api/pkg/resource/common"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestDaemonSetCellPropertiesAndSelector(t *testing.T) {
	created := metav1.NewTime(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	daemonSet := apps.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "default", UID: types.UID("uid"), CreationTimestamp: created}}
	cell := DaemonSetCell(daemonSet)
	for _, tc := range []struct {
		property dataselect.PropertyName
		want     dataselect.ComparableValue
	}{
		{dataselect.NameProperty, dataselect.StdComparableString("agent")},
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
	if selector.Namespace != "default" || selector.ResourceType != "daemonset" || selector.ResourceName != "agent" || selector.UID != "uid" {
		t.Errorf("GetResourceSelector() = %#v", selector)
	}
	if got := FromCells(ToCells([]apps.DaemonSet{daemonSet})); len(got) != 1 || got[0].Name != daemonSet.Name {
		t.Errorf("cell conversion = %#v", got)
	}
}

func TestDaemonSetClientBackedListDetailsPodsAndRestart(t *testing.T) {
	controlled := true
	daemonSet := &apps.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "default", UID: types.UID("daemonset-uid")},
		Spec: apps.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "agent"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "agent"}}, Spec: corev1.PodSpec{
				Containers:     []corev1.Container{{Name: "agent", Image: "agent:v1"}},
				InitContainers: []corev1.Container{{Name: "init", Image: "busybox:1"}},
			}},
		},
		Status: apps.DaemonSetStatus{DesiredNumberScheduled: 1, CurrentNumberScheduled: 1},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "agent-node-a", Namespace: "default", Labels: map[string]string{"app": "agent"}, OwnerReferences: []metav1.OwnerReference{{Kind: "DaemonSet", Name: daemonSet.Name, UID: daemonSet.UID, Controller: &controlled}}},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	client := fake.NewSimpleClientset(daemonSet, pod)

	list, err := GetDaemonSetList(client, common.NewSameNamespaceQuery("default"), dataselect.NoDataSelect, nil)
	if err != nil || list.ListMeta.TotalItems != 1 || len(list.DaemonSets) != 1 {
		t.Fatalf("GetDaemonSetList() = %#v, %v", list, err)
	}
	if list.DaemonSets[0].ContainerImages[0] != "agent:v1" || list.DaemonSets[0].InitContainerImages[0] != "busybox:1" {
		t.Errorf("container images = %#v / %#v", list.DaemonSets[0].ContainerImages, list.DaemonSets[0].InitContainerImages)
	}

	detail, err := GetDaemonSetDetail(client, nil, "default", "agent")
	if err != nil || detail.ObjectMeta.Name != "agent" || detail.LabelSelector == nil || detail.Pods.Current != 1 {
		t.Fatalf("GetDaemonSetDetail() = %#v, %v", detail, err)
	}
	pods, err := GetDaemonSetPods(client, nil, dataselect.NoDataSelect, "agent", "default")
	if err != nil || len(pods.Pods) != 1 || pods.Pods[0].ObjectMeta.Name != "agent-node-a" {
		t.Fatalf("GetDaemonSetPods() = %#v, %v", pods, err)
	}
	restarted, err := RestartDaemonSet(client, "default", "agent")
	if err != nil || restarted.Spec.Template.Annotations[RestartedAtAnnotationKey] == "" {
		t.Fatalf("RestartDaemonSet() = %#v, %v", restarted, err)
	}
	if _, err := GetDaemonSetDetail(client, nil, "default", "missing"); err == nil {
		t.Fatal("GetDaemonSetDetail() succeeded for missing daemon set")
	}
	if _, err := RestartDaemonSet(client, "default", "missing"); err == nil {
		t.Fatal("RestartDaemonSet() succeeded for missing daemon set")
	}
}
