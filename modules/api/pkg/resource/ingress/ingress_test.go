// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

package ingress

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"k8s.io/dashboard/api/pkg/resource/common"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestIngressListDetailAndDerivedFields(t *testing.T) {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "team-a"},
		Spec: networkingv1.IngressSpec{
			DefaultBackend: &networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: "default-svc"}},
			Rules: []networkingv1.IngressRule{
				{Host: "web.example.test", IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{Paths: []networkingv1.HTTPIngressPath{
					{Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: "service-svc"}}},
					{Backend: networkingv1.IngressBackend{Resource: &corev1.TypedLocalObjectReference{Kind: "Service", Name: "resource-svc"}}},
				}}}},
				{Host: "web.example.test"},
				{Host: ""},
			},
		},
		Status: networkingv1.IngressStatus{LoadBalancer: networkingv1.IngressLoadBalancerStatus{Ingress: []networkingv1.IngressLoadBalancerIngress{
			{Hostname: "lb.example.test"}, {IP: "192.0.2.1"}, {},
		}}},
	}
	client := fake.NewSimpleClientset(ingress)
	list, err := GetIngressList(client, common.NewSameNamespaceQuery("team-a"), dataselect.NoDataSelect)
	if err != nil || len(list.Items) != 1 {
		t.Fatalf("GetIngressList() = %#v, %v", list, err)
	}
	got := list.Items[0]
	if got.Name != "web" || len(got.Hosts) != 1 || got.Hosts[0] != "web.example.test" || len(got.Endpoints) != 3 {
		t.Fatalf("ingress list item = %#v", got)
	}
	if got.Endpoints[0].Host != "lb.example.test" || got.Endpoints[1].Host != "192.0.2.1" || got.Endpoints[2].Host != "" {
		t.Errorf("ingress endpoints = %#v", got.Endpoints)
	}
	detail, err := GetIngressDetail(client, "team-a", "web")
	if err != nil || detail.Name != "web" || len(detail.Spec.Rules) != 3 {
		t.Fatalf("GetIngressDetail() = %#v, %v", detail, err)
	}
	if _, err := GetIngressDetail(client, "team-a", "missing"); err == nil {
		t.Fatal("GetIngressDetail() accepted a missing ingress")
	}

	for _, serviceName := range []string{"default-svc", "service-svc", "resource-svc"} {
		if matches := FilterIngressByService([]networkingv1.Ingress{*ingress}, serviceName); len(matches) != 1 {
			t.Errorf("FilterIngressByService(%q) found %d matches", serviceName, len(matches))
		}
	}
	if matches := FilterIngressByService([]networkingv1.Ingress{*ingress}, "unrelated"); len(matches) != 0 {
		t.Errorf("FilterIngressByService(unrelated) = %d matches", len(matches))
	}
}

func TestIngressCellsAndBackendMatching(t *testing.T) {
	cell := IngressCell(networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "team-a"}})
	if got := cell.GetProperty(dataselect.NameProperty); got != dataselect.StdComparableString("web") {
		t.Errorf("name property = %v", got)
	}
	if got := cell.GetProperty(dataselect.NamespaceProperty); got != dataselect.StdComparableString("team-a") {
		t.Errorf("namespace property = %v", got)
	}
	if got := cell.GetProperty(dataselect.CreationTimestampProperty); got == nil {
		t.Error("creation timestamp property is nil")
	}
	if got := cell.GetProperty("other"); got != nil {
		t.Errorf("unsupported property = %v", got)
	}
	if got := fromCells(toCells([]networkingv1.Ingress{{ObjectMeta: metav1.ObjectMeta{Name: "web"}}})); len(got) != 1 || got[0].Name != "web" {
		t.Errorf("ingress cell conversion = %#v", got)
	}

	if ingressBackendMatchesServiceName(nil, "web") {
		t.Error("nil ingress backend matched a service")
	}
	if ingressBackendMatchesServiceName(&networkingv1.IngressBackend{Resource: &corev1.TypedLocalObjectReference{Kind: "ConfigMap", Name: "web"}}, "web") {
		t.Error("non-Service resource backend matched a service")
	}
}
