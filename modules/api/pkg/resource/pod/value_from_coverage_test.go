// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package pod

import (
	"encoding/base64"
	"testing"

	corev1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEvalValueFromResourceAndFieldReferences(t *testing.T) {
	container := &corev1.Container{Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
		corev1.ResourceCPU: resource.MustParse("1500m"),
	}}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-1"}, Spec: corev1.PodSpec{NodeName: "worker-1"}}
	emptyConfigMaps := &corev1.ConfigMapList{}
	emptySecrets := &corev1.SecretList{}

	tests := []struct {
		name      string
		source    *corev1.EnvVarSource
		container *corev1.Container
		pod       *corev1.Pod
		want      string
	}{
		{
			name:      "resource field",
			source:    &corev1.EnvVarSource{ResourceFieldRef: &corev1.ResourceFieldSelector{Resource: "limits.cpu"}},
			container: container,
			pod:       pod,
			want:      "2",
		},
		{
			name:      "zero cpu limit uses node allocatable label",
			source:    &corev1.EnvVarSource{ResourceFieldRef: &corev1.ResourceFieldSelector{Resource: "limits.cpu"}},
			container: &corev1.Container{},
			pod:       pod,
			want:      "node allocatable",
		},
		{
			name:      "zero memory limit uses node allocatable label",
			source:    &corev1.EnvVarSource{ResourceFieldRef: &corev1.ResourceFieldSelector{Resource: "limits.memory"}},
			container: &corev1.Container{},
			pod:       pod,
			want:      "node allocatable",
		},
		{
			name:      "unsupported resource field",
			source:    &corev1.EnvVarSource{ResourceFieldRef: &corev1.ResourceFieldSelector{Resource: "requests.storage"}},
			container: container,
			pod:       pod,
			want:      "",
		},
		{
			name:   "pod field",
			source: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "metadata.name"}},
			pod:    pod,
			want:   "pod-1",
		},
		{
			name:   "invalid api version",
			source: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{APIVersion: "not a version", FieldPath: "spec.nodeName"}},
			pod:    pod,
			want:   "",
		},
		{
			name:   "invalid pod field path",
			source: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "spec.unknown"}},
			pod:    pod,
			want:   "",
		},
		{
			name:   "empty source",
			source: &corev1.EnvVarSource{},
			pod:    pod,
			want:   "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := evalValueFrom(test.source, test.container, test.pod, emptyConfigMaps, emptySecrets)
			if got != test.want {
				t.Errorf("evalValueFrom() = %q, want %q", got, test.want)
			}
		})
	}

	configMapSource := &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: "settings"}, Key: "mode",
	}}
	if got := evalValueFrom(configMapSource, nil, nil, &corev1.ConfigMapList{}, emptySecrets); got != "" {
		t.Errorf("missing ConfigMap key value = %q, want empty", got)
	}
	secretSource := &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: "credentials"}, Key: "token",
	}}
	if got := evalValueFrom(secretSource, nil, nil, emptyConfigMaps, &corev1.SecretList{}); got != "" {
		t.Errorf("missing Secret key value = %q, want empty", got)
	}
	secret := &corev1.SecretList{Items: []corev1.Secret{{ObjectMeta: metav1.ObjectMeta{Name: "credentials"}, Data: map[string][]byte{"token": []byte("secret")}}}}
	if got, want := evalValueFrom(secretSource, nil, nil, emptyConfigMaps, secret), base64.StdEncoding.EncodeToString([]byte("secret")); got != want {
		t.Errorf("Secret key value = %q, want %q", got, want)
	}
}
