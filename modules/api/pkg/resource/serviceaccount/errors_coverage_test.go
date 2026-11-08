// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package serviceaccount

import (
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/dashboard/api/pkg/resource/common"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestServiceAccountReadersReturnClientFailures(t *testing.T) {
	wantErr := errors.New("service account request failed")

	t.Run("account list", func(t *testing.T) {
		client := fake.NewSimpleClientset()
		client.PrependReactor("list", "serviceaccounts", func(ktesting.Action) (bool, runtime.Object, error) {
			return true, nil, wantErr
		})
		if _, err := GetServiceAccountList(client, common.NewNamespaceQuery(nil), dataselect.NoDataSelect); !errors.Is(err, wantErr) {
			t.Fatalf("GetServiceAccountList() error = %v; want %v", err, wantErr)
		}
	})

	for _, test := range []struct {
		name string
		call func(*fake.Clientset) error
	}{
		{name: "image pull secret account lookup", call: func(client *fake.Clientset) error {
			_, err := GetServiceAccountImagePullSecrets(client, "default", "dashboard", dataselect.NoDataSelect)
			return err
		}},
		{name: "secret account lookup", call: func(client *fake.Clientset) error {
			_, err := GetServiceAccountSecrets(client, "default", "dashboard", dataselect.NoDataSelect)
			return err
		}},
		{name: "image pull secret list", call: func(client *fake.Clientset) error {
			_, err := GetServiceAccountImagePullSecrets(client, "default", "dashboard", dataselect.NoDataSelect)
			return err
		}},
		{name: "secret list", call: func(client *fake.Clientset) error {
			_, err := GetServiceAccountSecrets(client, "default", "dashboard", dataselect.NoDataSelect)
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			account := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "dashboard", Namespace: "default"},
				ImagePullSecrets: []corev1.LocalObjectReference{{Name: "registry"}},
				Secrets:          []corev1.ObjectReference{{Name: "token"}}}
			client := fake.NewSimpleClientset(account)
			if test.name == "image pull secret account lookup" || test.name == "secret account lookup" {
				client.PrependReactor("get", "serviceaccounts", func(ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, wantErr
				})
			} else {
				client.PrependReactor("list", "secrets", func(ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, wantErr
				})
			}
			if err := test.call(client); !errors.Is(err, wantErr) {
				t.Fatalf("service account reader error = %v; want %v", err, wantErr)
			}
		})
	}
}
