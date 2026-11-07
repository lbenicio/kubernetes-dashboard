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

package logs

import (
	"testing"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	dashboardtypes "k8s.io/dashboard/types"
)

func TestGetLogSourcesForPodAndController(t *testing.T) {
	controlled := true
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web-pod", Namespace: "default", UID: types.UID("pod-uid")}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "web"}}, InitContainers: []corev1.Container{{Name: "init"}}}}
	controller := &apps.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "web-rs", Namespace: "default", UID: types.UID("rs-uid")}, Spec: apps.ReplicaSetSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}}}}
	pod.Labels = map[string]string{"app": "web"}
	pod.OwnerReferences = []metav1.OwnerReference{{Kind: "ReplicaSet", Name: controller.Name, UID: controller.UID, Controller: &controlled}}
	client := fake.NewSimpleClientset(pod, controller)

	podSources, err := GetLogSources(client, "default", "web-pod", string(dashboardtypes.ResourceKindPod))
	if err != nil || len(podSources.PodNames) != 1 || podSources.PodNames[0] != "web-pod" || len(podSources.ContainerNames) != 1 || podSources.ContainerNames[0] != "web" || len(podSources.InitContainerNames) != 1 || podSources.InitContainerNames[0] != "init" {
		t.Fatalf("GetLogSources(pod) = %#v, %v", podSources, err)
	}
	controllerSources, err := GetLogSources(client, "default", controller.Name, string(dashboardtypes.ResourceKindReplicaSet))
	if err != nil || len(controllerSources.PodNames) != 1 || controllerSources.PodNames[0] != "web-pod" {
		t.Fatalf("GetLogSources(replicaset) = %#v, %v", controllerSources, err)
	}
	if _, err := GetLogSources(client, "default", "missing", string(dashboardtypes.ResourceKindPod)); err == nil {
		t.Fatal("GetLogSources() succeeded for missing pod")
	}
}
