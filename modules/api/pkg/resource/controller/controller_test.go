// Copyright 2017 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"reflect"
	"testing"

	apps "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"k8s.io/dashboard/types"
)

func TestGetPodNames(t *testing.T) {
	cases := []struct {
		pods          []v1.Pod
		expectedNames []string
	}{
		{
			pods:          nil,
			expectedNames: []string{},
		},
		{
			pods:          []v1.Pod{newPod("a"), newPod("b")},
			expectedNames: []string{"a", "b"},
		},
	}

	for _, c := range cases {
		actual := getPodNames(c.pods)
		if !reflect.DeepEqual(actual, c.expectedNames) {
			t.Errorf("GetPodNames(%+v) == %+v, expected %+v",
				c.pods, actual, c.expectedNames)
		}
	}

}

func TestNewResourceController(t *testing.T) {
	pod := v1.Pod{
		TypeMeta: meta.TypeMeta{
			Kind:       "pod",
			APIVersion: "v1",
		},
		ObjectMeta: meta.ObjectMeta{
			Name:      "test-name",
			Namespace: "default",
		}}
	cli := fake.NewClientset(&pod)

	ctrl, err := NewResourceController(
		meta.OwnerReference{
			Kind: types.ResourceKindPod,
			Name: "test-name",
		}, "default", cli)

	if err != nil {
		t.Fatal("Returned Error finding pod")
	}
	podCtrl, ok := ctrl.(PodController)
	if !ok {
		t.Fatal("Returned value is not pod controller")
	}
	if podCtrl.Name != "test-name" {
		t.Fatal("Returned invalid pod name")
	}
	_, _ = NewResourceController(
		meta.OwnerReference{
			Kind: types.ResourceKindPod,
			Name: "test-name",
		}, "default", fake.NewClientset())
	podCtrl.Get([]v1.Pod{pod}, []v1.Event{})
	podCtrl.UID()
	podCtrl.GetLogSources([]v1.Pod{pod})
}

func TestNewResourceControllerSupportsEachOwnerKind(t *testing.T) {
	tests := []struct {
		name     string
		kind     string
		object   runtime.Object
		uid      string
		wantType string
	}{
		{name: "job", kind: types.ResourceKindJob, object: &batch.Job{TypeMeta: meta.TypeMeta{Kind: "Job", APIVersion: "batch/v1"}, ObjectMeta: meta.ObjectMeta{Name: "job", Namespace: "ns", UID: "job-uid"}}, uid: "job-uid", wantType: types.ResourceKindJob},
		{name: "pod", kind: types.ResourceKindPod, object: &v1.Pod{TypeMeta: meta.TypeMeta{Kind: "Pod", APIVersion: "v1"}, ObjectMeta: meta.ObjectMeta{Name: "pod", Namespace: "ns", UID: "pod-uid"}}, uid: "pod-uid", wantType: types.ResourceKindPod},
		{name: "replicaset", kind: types.ResourceKindReplicaSet, object: &apps.ReplicaSet{TypeMeta: meta.TypeMeta{Kind: "ReplicaSet", APIVersion: "apps/v1"}, ObjectMeta: meta.ObjectMeta{Name: "replicaset", Namespace: "ns", UID: "replicaset-uid"}}, uid: "replicaset-uid", wantType: types.ResourceKindReplicaSet},
		{name: "replicationcontroller", kind: types.ResourceKindReplicationController, object: &v1.ReplicationController{TypeMeta: meta.TypeMeta{Kind: "ReplicationController", APIVersion: "v1"}, ObjectMeta: meta.ObjectMeta{Name: "replicationcontroller", Namespace: "ns", UID: "rc-uid"}}, uid: "rc-uid", wantType: types.ResourceKindReplicationController},
		{name: "daemonset", kind: types.ResourceKindDaemonSet, object: &apps.DaemonSet{TypeMeta: meta.TypeMeta{Kind: "DaemonSet", APIVersion: "apps/v1"}, ObjectMeta: meta.ObjectMeta{Name: "daemonset", Namespace: "ns", UID: "daemonset-uid"}}, uid: "daemonset-uid", wantType: types.ResourceKindDaemonSet},
		{name: "statefulset", kind: types.ResourceKindStatefulSet, object: &apps.StatefulSet{TypeMeta: meta.TypeMeta{Kind: "StatefulSet", APIVersion: "apps/v1"}, ObjectMeta: meta.ObjectMeta{Name: "statefulset", Namespace: "ns", UID: "statefulset-uid"}}, uid: "statefulset-uid", wantType: types.ResourceKindStatefulSet},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			object := tc.object
			controller, err := NewResourceController(meta.OwnerReference{Kind: tc.kind, Name: tc.kind}, "ns", fake.NewClientset(object))
			if err != nil {
				t.Fatalf("NewResourceController() error = %v", err)
			}
			if got := string(controller.UID()); got != string(tc.uid) {
				t.Errorf("UID() = %q; want %q", got, tc.uid)
			}
			owner := controller.Get(nil, nil)
			if string(owner.TypeMeta.Kind) != tc.wantType {
				t.Errorf("Get().TypeMeta.Kind = %q; want %q", owner.TypeMeta.Kind, tc.wantType)
			}
			_ = controller.GetLogSources(nil)
		})
	}
}

func TestNewResourceControllerReportsUnknownAndMissingOwners(t *testing.T) {
	if controller, err := NewResourceController(meta.OwnerReference{Kind: "Unknown", Name: "missing"}, "ns", fake.NewClientset()); err == nil || controller != nil {
		t.Fatalf("unknown kind returned %v, %v; want nil controller and error", controller, err)
	}
	for _, kind := range []string{
		types.ResourceKindJob,
		types.ResourceKindPod,
		types.ResourceKindReplicaSet,
		types.ResourceKindReplicationController,
		types.ResourceKindDaemonSet,
		types.ResourceKindStatefulSet,
	} {
		t.Run(kind, func(t *testing.T) {
			if controller, err := NewResourceController(meta.OwnerReference{Kind: kind, Name: "missing"}, "ns", fake.NewClientset()); err == nil || controller != nil {
				t.Fatalf("missing %s returned %v, %v; want nil controller and error", kind, controller, err)
			}
		})
	}
}

func TestReplicationControllerControllerHandlesEmptyTemplate(t *testing.T) {
	controller := ReplicationControllerController(v1.ReplicationController{})
	owner := controller.Get(nil, nil)
	if string(owner.TypeMeta.Kind) != types.ResourceKindReplicationController {
		t.Errorf("Get().TypeMeta.Kind = %q", owner.TypeMeta.Kind)
	}
	if owner.ContainerImages != nil || owner.InitContainerImages != nil {
		t.Errorf("empty pod template images = %v and %v; want nil", owner.ContainerImages, owner.InitContainerImages)
	}
	sources := controller.GetLogSources(nil)
	if sources.ContainerNames != nil || sources.InitContainerNames != nil {
		t.Errorf("empty pod template names = %v and %v; want nil", sources.ContainerNames, sources.InitContainerNames)
	}

	template := &v1.PodTemplateSpec{Spec: v1.PodSpec{Containers: []v1.Container{{Name: "web"}}}}
	if got := getReplicationControllerPodSpec(template); got != &template.Spec || len(got.Containers) != 1 || got.Containers[0].Name != "web" {
		t.Errorf("getReplicationControllerPodSpec(non-empty) = %#v", got)
	}
}

func newPod(name string) v1.Pod {
	return v1.Pod{ObjectMeta: meta.ObjectMeta{Name: name}}
}
