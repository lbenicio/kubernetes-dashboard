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

	corev1 "k8s.io/api/core/v1"
)

func TestToContainerStateMapsAllContainerStates(t *testing.T) {
	for _, test := range []struct {
		name  string
		state corev1.ContainerState
		want  ContainerState
	}{
		{name: "waiting", state: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{}}, want: Waiting},
		{name: "terminated successfully", state: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{}}, want: Terminated},
		{name: "terminated with failure", state: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 1}}, want: Failed},
		{name: "running", state: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}, want: Running},
		{name: "unknown", state: corev1.ContainerState{}, want: Unknown},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := toContainerState(test.state); got != test.want {
				t.Fatalf("toContainerState() = %q; want %q", got, test.want)
			}
		})
	}
}
