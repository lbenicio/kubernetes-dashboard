// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

package core_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"k8s.io/dashboard/client/cache"
	"k8s.io/dashboard/client/cache/client/common"
	coreclient "k8s.io/dashboard/client/cache/client/core"
)

func TestCachedCoreClientListWrappers(t *testing.T) {
	client, err := coreclient.NewClient(testRESTConfig(), nil, coreOptions())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	tests := []struct {
		name string
		list func() error
	}{
		{name: "pods", list: func() error {
			_, err := client.Pods("default").List(context.Background(), metav1.ListOptions{})
			return err
		}},
		{name: "configmaps", list: func() error {
			_, err := client.ConfigMaps("default").List(context.Background(), metav1.ListOptions{})
			return err
		}},
		{name: "secrets", list: func() error {
			_, err := client.Secrets("default").List(context.Background(), metav1.ListOptions{})
			return err
		}},
		{name: "namespaces", list: func() error {
			_, err := client.Namespaces().List(context.Background(), metav1.ListOptions{})
			return err
		}},
		{name: "nodes", list: func() error { _, err := client.Nodes().List(context.Background(), metav1.ListOptions{}); return err }},
		{name: "persistent volumes", list: func() error {
			_, err := client.PersistentVolumes().List(context.Background(), metav1.ListOptions{})
			return err
		}},
		{name: "persistent volume claims", list: func() error {
			_, err := client.PersistentVolumeClaims("default").List(context.Background(), metav1.ListOptions{})
			return err
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cache.Clear[corev1.PodList]()
			cache.Clear[corev1.ConfigMapList]()
			cache.Clear[corev1.SecretList]()
			cache.Clear[corev1.NamespaceList]()
			cache.Clear[corev1.NodeList]()
			cache.Clear[corev1.PersistentVolumeList]()
			cache.Clear[corev1.PersistentVolumeClaimList]()
			if err := tc.list(); err != nil {
				t.Fatalf("List() error = %v", err)
			}
		})
	}
}

func testRESTConfig() *rest.Config {
	return &rest.Config{
		Host: "https://kubernetes.example.test",
		WrapTransport: func(http.RoundTripper) http.RoundTripper {
			return roundTripFunc(func(request *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"items":[]}`)),
					Request:    request,
				}, nil
			})
		},
	}
}

func coreOptions() common.CachedClientOptions {
	return common.CachedClientOptions{Token: "core-client-test-token"}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (in roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return in(request)
}
