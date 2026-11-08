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

package node

import (
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestNodePresentationHelpersMapImagesAndConditions(t *testing.T) {
	probeTime := metav1.Now()
	transitionTime := metav1.NewTime(probeTime.Add(-1))
	node := corev1.Node{Status: corev1.NodeStatus{
		Images: []corev1.ContainerImage{{Names: []string{"registry/web:v1", "registry/web:latest"}}, {Names: []string{"registry/agent:v2"}}, {}},
		Conditions: []corev1.NodeCondition{{
			Type: corev1.NodeReady, Status: corev1.ConditionTrue,
			LastHeartbeatTime: probeTime, LastTransitionTime: transitionTime,
			Reason: "KubeletReady", Message: "ready",
		}},
	}}
	if got := getContainerImages(node); len(got) != 3 || got[0] != "registry/web:v1" || got[1] != "registry/web:latest" || got[2] != "registry/agent:v2" {
		t.Errorf("getContainerImages() = %#v", got)
	}
	conditions := getNodeConditions(node)
	if len(conditions) != 1 || conditions[0].Type != string(corev1.NodeReady) || conditions[0].Status != corev1.ConditionTrue ||
		!conditions[0].LastProbeTime.Equal(&probeTime) || !conditions[0].LastTransitionTime.Equal(&transitionTime) ||
		conditions[0].Reason != "KubeletReady" || conditions[0].Message != "ready" {
		t.Errorf("getNodeConditions() = %#v", conditions)
	}
	if got := getNodeConditions(corev1.Node{}); got != nil {
		t.Errorf("getNodeConditions(empty) = %#v; want nil", got)
	}
}

func TestGetNodePodsQueriesAndHandlesFailures(t *testing.T) {
	node := corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-a"}}
	pod := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"}, Spec: corev1.PodSpec{NodeName: node.Name}}
	t.Run("lists pods on node", func(t *testing.T) {
		client := fake.NewSimpleClientset(&node, &pod)
		got, err := getNodePods(client, node)
		if err != nil || len(got.Items) != 1 || got.Items[0].Name != pod.Name {
			t.Fatalf("getNodePods() = %#v, %v", got, err)
		}
	})
	t.Run("rejects invalid field selector", func(t *testing.T) {
		if _, err := getNodePods(fake.NewSimpleClientset(), corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "invalid,name"}}); err == nil {
			t.Fatal("getNodePods() accepted a node name that makes an invalid field selector")
		}
	})
	t.Run("returns list errors", func(t *testing.T) {
		client := fake.NewSimpleClientset()
		client.PrependReactor("list", "pods", func(clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("pod listing failed")
		})
		if _, err := getNodePods(client, node); err == nil {
			t.Fatal("getNodePods() error = nil; want list error")
		}
	})
}

func TestGetNodePodsReturnsRelatedPodsAndPropagatesRequestErrors(t *testing.T) {
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-a"}}
	query := dataselect.NoDataSelect
	if _, err := GetNodePods(fake.NewSimpleClientset(), nil, query, node.Name); err == nil {
		t.Fatal("GetNodePods() succeeded for a missing node")
	}

	t.Run("pod list error", func(t *testing.T) {
		client := fake.NewSimpleClientset(node)
		client.PrependReactor("list", "pods", func(clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("pod listing failed")
		})
		if _, err := GetNodePods(client, nil, query, node.Name); err == nil {
			t.Fatal("GetNodePods() error = nil; want pod list error")
		}
	})

	t.Run("event list error", func(t *testing.T) {
		client := fake.NewSimpleClientset(node)
		client.PrependReactor("list", "events", func(clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("event listing failed")
		})
		if _, err := GetNodePods(client, nil, query, node.Name); err == nil {
			t.Fatal("GetNodePods() error = nil; want event list error")
		}
	})

	client := fake.NewSimpleClientset(node, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"}, Spec: corev1.PodSpec{NodeName: node.Name}})
	got, err := GetNodePods(client, nil, query, node.Name)
	if err != nil || len(got.Pods) != 1 || got.Pods[0].ObjectMeta.Name != "web" {
		t.Fatalf("GetNodePods() = %#v, %v", got, err)
	}
	if got.CumulativeMetrics == nil || got.Errors == nil {
		t.Errorf("GetNodePods() should initialize empty metrics/errors: %#v", got)
	}
}

func TestToNodeMapsReadyStateAndAllocatedResources(t *testing.T) {
	node := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-a"},
		Status: corev1.NodeStatus{
			Conditions:  []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
			Allocatable: corev1.ResourceList{},
		},
	}
	got := toNode(node, &corev1.PodList{})
	if got.ObjectMeta.Name != "worker-a" || got.TypeMeta.Kind != "node" || got.Ready != corev1.ConditionTrue || got.AllocatedResources.AllocatedPods != 0 {
		t.Errorf("toNode() = %#v", got)
	}
}
