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

package node

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetNodeAllocatedResources(t *testing.T) {
	node := corev1.Node{Status: corev1.NodeStatus{
		Allocatable: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("2"),
			corev1.ResourceMemory: resource.MustParse("4Gi"),
		},
		Capacity: corev1.ResourceList{corev1.ResourcePods: resource.MustParse("10")},
	}}
	pods := &corev1.PodList{Items: []corev1.Pod{
		{ObjectMeta: metav1.ObjectMeta{Name: "one"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m"), corev1.ResourceMemory: resource.MustParse("512Mi")},
			Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("1Gi")},
		}}}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "two"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("250m"), corev1.ResourceMemory: resource.MustParse("256Mi")},
			Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m"), corev1.ResourceMemory: resource.MustParse("512Mi")},
		}}}}},
	}}

	got, err := getNodeAllocatedResources(node, pods)
	if err != nil {
		t.Fatalf("getNodeAllocatedResources() error = %v", err)
	}
	if got.CPURequests != 750 || got.CPURequestsFraction != 37.5 || got.CPULimits != 1500 || got.CPULimitsFraction != 75 {
		t.Errorf("CPU allocation = %#v", got)
	}
	if got.MemoryRequests != 805306368 || got.MemoryRequestsFraction != 18.75 ||
		got.MemoryLimits != 1610612736 || got.MemoryLimitsFraction != 37.5 {
		t.Errorf("memory allocation = %#v", got)
	}
	if got.CPUCapacity != 2000 || got.MemoryCapacity != 4294967296 ||
		got.AllocatedPods != 2 || got.PodCapacity != 10 || got.PodFraction != 20 {
		t.Errorf("node capacities = %#v", got)
	}
}

func TestGetNodeAllocatedResourcesWithZeroCapacity(t *testing.T) {
	got, err := getNodeAllocatedResources(corev1.Node{}, &corev1.PodList{})
	if err != nil {
		t.Fatalf("getNodeAllocatedResources() error = %v", err)
	}
	if got.CPURequestsFraction != 0 || got.CPULimitsFraction != 0 || got.MemoryRequestsFraction != 0 ||
		got.MemoryLimitsFraction != 0 || got.PodFraction != 0 {
		t.Errorf("zero-capacity allocation fractions = %#v", got)
	}
}

func TestGetNodeConditionStatus(t *testing.T) {
	node := corev1.Node{Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{
		{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionTrue},
		{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
	}}}
	if got := getNodeConditionStatus(node, corev1.NodeReady); got != corev1.ConditionFalse {
		t.Errorf("ready condition = %q, want False", got)
	}
	if got := getNodeConditionStatus(node, corev1.NodeDiskPressure); got != corev1.ConditionUnknown {
		t.Errorf("missing condition = %q, want Unknown", got)
	}
}
