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

package pod

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"

	"k8s.io/dashboard/api/pkg/resource/common"
	"k8s.io/dashboard/types"
)

func TestGetPodStatusLifecycleBranches(t *testing.T) {
	now := metav1.NewTime(time.Now())
	started := true
	restartAlways := corev1.ContainerRestartPolicyAlways

	tests := []struct {
		name string
		pod  corev1.Pod
		want string
	}{
		{
			name: "scheduling gated condition overrides phase",
			pod: corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodPending, Conditions: []corev1.PodCondition{{
				Type: corev1.PodScheduled, Reason: corev1.PodReasonSchedulingGated,
			}}}},
			want: string(corev1.PodReasonSchedulingGated),
		},
		{
			name: "init container failure reason",
			pod: corev1.Pod{Spec: corev1.PodSpec{InitContainers: []corev1.Container{{Name: "init"}}}, Status: corev1.PodStatus{
				Phase: corev1.PodPending, InitContainerStatuses: []corev1.ContainerStatus{{Name: "init", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "InvalidImageName", ExitCode: 1}}}},
			}},
			want: "Init:InvalidImageName",
		},
		{
			name: "init container signal",
			pod: corev1.Pod{Spec: corev1.PodSpec{InitContainers: []corev1.Container{{Name: "init"}}}, Status: corev1.PodStatus{
				Phase: corev1.PodPending, InitContainerStatuses: []corev1.ContainerStatus{{Name: "init", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Signal: 9, ExitCode: 137}}}},
			}},
			want: "Init: Signal 9",
		},
		{
			name: "init container exit code",
			pod: corev1.Pod{Spec: corev1.PodSpec{InitContainers: []corev1.Container{{Name: "init"}}}, Status: corev1.PodStatus{
				Phase: corev1.PodPending, InitContainerStatuses: []corev1.ContainerStatus{{Name: "init", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 2}}}},
			}},
			want: "Init: ExitCode 2",
		},
		{
			name: "init waiting reason",
			pod: corev1.Pod{Spec: corev1.PodSpec{InitContainers: []corev1.Container{{Name: "init"}}}, Status: corev1.PodStatus{
				Phase: corev1.PodPending, InitContainerStatuses: []corev1.ContainerStatus{{Name: "init", State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}}}},
			}},
			want: "Init: ImagePullBackOff",
		},
		{
			name: "init progress",
			pod: corev1.Pod{Spec: corev1.PodSpec{InitContainers: []corev1.Container{{Name: "init"}}}, Status: corev1.PodStatus{
				Phase: corev1.PodPending, InitContainerStatuses: []corev1.ContainerStatus{{Name: "init", State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "PodInitializing"}}}},
			}},
			want: "Init: 0/1",
		},
		{
			name: "running restartable init container is initialized",
			pod: corev1.Pod{Spec: corev1.PodSpec{InitContainers: []corev1.Container{{Name: "sidecar", RestartPolicy: &restartAlways}}}, Status: corev1.PodStatus{
				Phase: corev1.PodPending, InitContainerStatuses: []corev1.ContainerStatus{{Name: "sidecar", Started: &started, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}},
			}},
			want: string(corev1.PodPending),
		},
		{
			name: "waiting application container",
			pod: corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
			}}}},
			want: "CrashLoopBackOff",
		},
		{
			name: "terminated application reason",
			pod: corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodFailed, ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "Error", ExitCode: 1}},
			}}}},
			want: "Error",
		},
		{
			name: "terminated application signal",
			pod: corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodFailed, ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Signal: 9}},
			}}}},
			want: "Signal: 9",
		},
		{
			name: "terminated application exit code",
			pod: corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodFailed, ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 1}},
			}}}},
			want: "ExitCode: 1",
		},
		{
			name: "completed pod with ready running container",
			pod: corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodSucceeded, Reason: "Completed", Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}, ContainerStatuses: []corev1.ContainerStatus{{
				Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
			}}}},
			want: string(corev1.PodRunning),
		},
		{
			name: "completed pod with unready running container",
			pod: corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodSucceeded, Reason: "Completed", ContainerStatuses: []corev1.ContainerStatus{{
				Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
			}}}},
			want: "NotReady",
		},
		{
			name: "unreachable pod being deleted",
			pod: corev1.Pod{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now}, Status: corev1.PodStatus{
				Phase: corev1.PodRunning, Reason: types.NodeUnreachablePodReason,
			}},
			want: string(corev1.PodUnknown),
		},
		{
			name: "non-terminal pod being deleted",
			pod:  corev1.Pod{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
			want: "Terminating",
		},
		{
			name: "terminal pod being deleted keeps its phase",
			pod:  corev1.Pod{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now}, Status: corev1.PodStatus{Phase: corev1.PodSucceeded}},
			want: string(corev1.PodSucceeded),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := getPodStatus(test.pod); got != test.want {
				t.Errorf("getPodStatus() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestGetPodStatusPhase(t *testing.T) {
	now := metav1.Now()
	readyAndInitialized := []corev1.PodCondition{
		{Type: corev1.PodInitialized, Status: corev1.ConditionTrue},
		{Type: corev1.PodReady, Status: corev1.ConditionTrue},
	}
	tests := []struct {
		name     string
		pod      corev1.Pod
		warnings []common.Event
		want     corev1.PodPhase
	}{
		{name: "failed phase is terminal", pod: corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodFailed}}, want: corev1.PodFailed},
		{name: "succeeded phase is terminal", pod: corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodSucceeded}}, want: corev1.PodSucceeded},
		{name: "ready initialized running pod", pod: corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: readyAndInitialized}}, want: corev1.PodRunning},
		{name: "warning marks nonterminal pod failed", pod: corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodPending}}, warnings: []common.Event{{Reason: "FailedScheduling"}}, want: corev1.PodFailed},
		{name: "deleting pod is terminating", pod: corev1.Pod{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now}, Status: corev1.PodStatus{Phase: corev1.PodRunning}}, want: "Terminating"},
		{name: "unready pod has no derived phase", pod: corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodRunning}}, want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := getPodStatusPhase(test.pod, test.warnings); got != test.want {
				t.Errorf("getPodStatusPhase() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestGetStatusCountsPodPhases(t *testing.T) {
	if status := getStatus(nil, nil); status != (common.ResourceStatus{}) {
		t.Errorf("getStatus(nil) = %#v", status)
	}
	list := &corev1.PodList{Items: []corev1.Pod{
		{ObjectMeta: metav1.ObjectMeta{UID: apimachinerytypes.UID("running")}, Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{
			{Type: corev1.PodInitialized, Status: corev1.ConditionTrue}, {Type: corev1.PodReady, Status: corev1.ConditionTrue},
		}}},
		{ObjectMeta: metav1.ObjectMeta{UID: apimachinerytypes.UID("failed")}, Status: corev1.PodStatus{Phase: corev1.PodFailed}},
		{ObjectMeta: metav1.ObjectMeta{UID: apimachinerytypes.UID("succeeded")}, Status: corev1.PodStatus{Phase: corev1.PodSucceeded}},
		{ObjectMeta: metav1.ObjectMeta{UID: apimachinerytypes.UID("terminating"), DeletionTimestamp: ptrToTime(metav1.Now())}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
	}}
	status := getStatus(list, nil)
	if status.Running != 1 || status.Failed != 1 || status.Succeeded != 1 || status.Terminating != 1 {
		t.Errorf("getStatus() = %#v", status)
	}
}

func ptrToTime(value metav1.Time) *metav1.Time { return &value }
