// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package secret

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetSecretDetailClientFlow(t *testing.T) {
	client := fake.NewClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "credentials", Namespace: "team-a"},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{"token": []byte("secret-value")},
	})

	got, err := GetSecretDetail(client, "team-a", "credentials")
	if err != nil {
		t.Fatalf("GetSecretDetail() error = %v", err)
	}
	if got.ObjectMeta.Name != "credentials" || got.ObjectMeta.Namespace != "team-a" ||
		got.Data["token"] == nil || string(got.Data["token"]) != "secret-value" {
		t.Errorf("GetSecretDetail() = %#v", got)
	}

	if _, err := GetSecretDetail(client, "team-a", "missing"); err == nil {
		t.Error("GetSecretDetail() succeeded for a missing secret")
	}
}
