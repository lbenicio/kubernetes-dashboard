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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/dashboard/api/pkg/resource/common"
)

func TestGetStatusClassifiesDaemonSets(t *testing.T) {
	if got := getStatus(nil, nil, nil); got != (common.ResourceStatus{}) {
		t.Fatalf("getStatus(nil) = %#v; want empty status", got)
	}

	sets := []appsv1.DaemonSet{
		{ObjectMeta: metav1.ObjectMeta{Name: "failed", UID: "ds-failed"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "pending", UID: "ds-pending"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "running", UID: "ds-running"}},
	}
	pods := []corev1.Pod{
		ownedDaemonSetPod("pod-failed", "ds-failed", corev1.PodFailed),
		ownedDaemonSetPod("pod-pending", "ds-pending", corev1.PodPending),
		ownedDaemonSetPod("pod-running", "ds-running", corev1.PodRunning),
	}
	events := []corev1.Event{{
		Type:           corev1.EventTypeWarning,
		Reason:         "FailedScheduling",
		InvolvedObject: corev1.ObjectReference{UID: "pod-failed"},
	}}

	got := getStatus(&appsv1.DaemonSetList{Items: sets}, pods, events)
	want := common.ResourceStatus{Failed: 1, Pending: 1, Running: 1}
	if got != want {
		t.Fatalf("getStatus() = %#v; want %#v", got, want)
	}
}

func ownedDaemonSetPod(name, ownerUID string, phase corev1.PodPhase) corev1.Pod {
	controller := true
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			UID:  types.UID(name),
			OwnerReferences: []metav1.OwnerReference{{
				UID: types.UID(ownerUID), Controller: &controller,
			}},
		},
		Status: corev1.PodStatus{Phase: phase},
	}
}
