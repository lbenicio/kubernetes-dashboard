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
