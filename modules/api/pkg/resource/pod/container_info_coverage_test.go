// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package pod

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExtractContainerInfo(t *testing.T) {
	readOnly := true
	container := corev1.Container{
		Name:    "web",
		Image:   "example.test/web:v1",
		Command: []string{"/entrypoint"},
		Args:    []string{"serve"},
		Env: []corev1.EnvVar{
			{Name: "MODE", Value: "production"},
			{Name: "CONFIG_VALUE", ValueFrom: &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "settings"}, Key: "mode",
			}}},
			{Name: "SECRET_VALUE", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "credentials"}, Key: "token",
			}}},
		},
		VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/data", ReadOnly: readOnly, SubPath: "web"}},
		Resources:    corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("250m")}},
	}
	container.RestartPolicy = nil
	probe := &corev1.Probe{}
	container.ReadinessProbe = probe
	securityContext := &corev1.SecurityContext{RunAsNonRoot: boolPointer(true)}
	container.SecurityContext = securityContext
	started := true
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{container},
			Volumes:    []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}},
		},
		Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
			Name: "web", Ready: true, Started: &started, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
		}}},
	}
	configMaps := &corev1.ConfigMapList{Items: []corev1.ConfigMap{{
		ObjectMeta: metav1.ObjectMeta{Name: "settings"}, Data: map[string]string{"mode": "production"},
	}}}
	secrets := &corev1.SecretList{Items: []corev1.Secret{{
		ObjectMeta: metav1.ObjectMeta{Name: "credentials"}, Data: map[string][]byte{"token": []byte("secret")},
	}}}

	got := extractContainerInfo([]corev1.Container{container}, pod, configMaps, secrets)
	if len(got) != 1 {
		t.Fatalf("extractContainerInfo() returned %d containers, want 1", len(got))
	}
	actual := got[0]
	if actual.Name != "web" || actual.Image != "example.test/web:v1" || actual.State != Running || actual.Status == nil ||
		actual.Status.Name != "web" || actual.ReadinessProbe != probe || actual.SecurityContext != securityContext {
		t.Errorf("container info = %#v", actual)
	}
	if len(actual.Env) != 3 || actual.Env[0].Value != "production" || actual.Env[1].Value != "production" || actual.Env[2].Value != "c2VjcmV0" {
		t.Errorf("container environment = %#v", actual.Env)
	}
	if len(actual.VolumeMounts) != 1 || actual.VolumeMounts[0].MountPath != "/data" || !actual.VolumeMounts[0].ReadOnly ||
		actual.VolumeMounts[0].Volume.EmptyDir == nil || actual.Resources.Requests.Cpu().String() != "250m" {
		t.Errorf("container mounts/resources = %#v", actual)
	}
}

func boolPointer(value bool) *bool { return &value }
