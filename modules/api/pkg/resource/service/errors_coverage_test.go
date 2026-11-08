// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package service

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

func TestServiceDetailAndPodsReturnKubernetesFailures(t *testing.T) {
	wantErr := errors.New("service request failed")
	client := fake.NewSimpleClientset()
	client.PrependReactor("get", "services", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, wantErr
	})
	if _, err := GetServiceDetail(client, "default", "web"); !errors.Is(err, wantErr) {
		t.Fatalf("GetServiceDetail() error = %v, want %v", err, wantErr)
	}
	if _, err := GetServicePods(client, nil, "default", "web", dataselect.NoDataSelect); !errors.Is(err, wantErr) {
		t.Fatalf("GetServicePods() service error = %v, want %v", err, wantErr)
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       corev1.ServiceSpec{Selector: map[string]string{"app": "web"}},
	}
	client = fake.NewSimpleClientset(svc)
	client.PrependReactor("list", "pods", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, wantErr
	})
	if _, err := GetServicePods(client, nil, "default", "web", dataselect.NoDataSelect); !errors.Is(err, wantErr) {
		t.Fatalf("GetServicePods() pod list error = %v, want %v", err, wantErr)
	}

	client = fake.NewSimpleClientset(svc)
	client.PrependReactor("list", "endpointslices", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, wantErr
	})
	if _, err := GetServiceDetail(client, "default", "web"); !errors.Is(err, wantErr) {
		t.Fatalf("GetServiceDetail() endpoint list error = %v, want %v", err, wantErr)
	}
}

func TestRelatedServiceLookupsReturnKubernetesFailures(t *testing.T) {
	wantErr := errors.New("related resource request failed")
	for _, test := range []struct {
		name string
		call func(*fake.Clientset) error
	}{
		{name: "service events", call: func(client *fake.Clientset) error {
			_, err := GetServiceEvents(client, dataselect.NoDataSelect, "default", "web")
			return err
		}},
		{name: "service ingress list", call: func(client *fake.Clientset) error {
			_, err := GetServiceIngressList(client, dataselect.NoDataSelect, "default", "web")
			return err
		}},
		{name: "service list", call: func(client *fake.Clientset) error {
			_, err := GetServiceList(client, common.NewNamespaceQuery([]string{"default"}), dataselect.NoDataSelect)
			return err
		}},
		{name: "service pods events", call: func(client *fake.Clientset) error {
			_, err := GetServicePods(client, nil, "default", "web", dataselect.NoDataSelect)
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
				Spec:       corev1.ServiceSpec{Selector: map[string]string{"app": "web"}},
			}
			client := fake.NewSimpleClientset(service)
			resource := "events"
			if test.name == "service ingress list" {
				resource = "ingresses"
			} else if test.name == "service list" {
				resource = "services"
			} else if test.name == "service pods events" {
				client.PrependReactor("list", "pods", func(ktesting.Action) (bool, runtime.Object, error) {
					return true, &corev1.PodList{}, nil
				})
			}
			client.PrependReactor("list", resource, func(ktesting.Action) (bool, runtime.Object, error) {
				return true, nil, wantErr
			})
			if err := test.call(client); !errors.Is(err, wantErr) {
				t.Fatalf("%s error = %v, want %v", test.name, err, wantErr)
			}
		})
	}
}

func TestGetServicePodsReturnsAnEmptyListWhenServiceHasNoSelector(t *testing.T) {
	service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "external", Namespace: "default"}}
	got, err := GetServicePods(fake.NewSimpleClientset(service), nil, "default", service.Name, dataselect.NoDataSelect)
	if err != nil {
		t.Fatalf("GetServicePods() error = %v", err)
	}
	if got == nil || len(got.Pods) != 0 || len(got.CumulativeMetrics) != 0 {
		t.Fatalf("GetServicePods() = %#v; want an empty initialized list", got)
	}
}
