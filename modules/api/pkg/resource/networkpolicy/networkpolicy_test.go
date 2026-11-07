// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

package networkpolicy

import (
	"context"
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"

	"k8s.io/dashboard/api/pkg/resource/common"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestGetNetworkPolicyListAndDetail(t *testing.T) {
	policy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "allow-web", Namespace: "team-a"},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress:     []networkingv1.NetworkPolicyIngressRule{{}},
		},
	}
	client := fake.NewSimpleClientset(policy)
	list, err := GetNetworkPolicyList(client, common.NewSameNamespaceQuery("team-a"), dataselect.NoDataSelect)
	if err != nil || len(list.Items) != 1 || list.Items[0].Name != "allow-web" {
		t.Fatalf("GetNetworkPolicyList() = %#v, %v", list, err)
	}
	detail, err := GetNetworkPolicyDetail(client, "team-a", "allow-web")
	if err != nil || detail.Name != "allow-web" || detail.PodSelector.MatchLabels["app"] != "web" || len(detail.Ingress) != 1 {
		t.Fatalf("GetNetworkPolicyDetail() = %#v, %v", detail, err)
	}
	if _, err := GetNetworkPolicyDetail(client, "team-a", "missing"); err == nil {
		t.Fatal("GetNetworkPolicyDetail() accepted a missing policy")
	}
}

func TestNetworkPolicyDataCells(t *testing.T) {
	policy := networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "policy", Namespace: "team-a"}}
	cell := ServiceAccountCell(policy)
	if got := cell.GetProperty(dataselect.NameProperty); got != dataselect.StdComparableString("policy") {
		t.Errorf("name property = %v", got)
	}
	if got := cell.GetProperty(dataselect.NamespaceProperty); got != dataselect.StdComparableString("team-a") {
		t.Errorf("namespace property = %v", got)
	}
	if got := cell.GetProperty(dataselect.CreationTimestampProperty); got == nil {
		t.Error("creation timestamp property is nil")
	}
	if got := cell.GetProperty("unsupported"); got != nil {
		t.Errorf("unsupported property = %v, want nil", got)
	}
	cells := toCells([]networkingv1.NetworkPolicy{policy})
	if got := fromCells(cells); len(got) != 1 || got[0].Name != "policy" {
		t.Errorf("cell conversion = %#v", got)
	}
}

func TestGetNetworkPolicyListReturnsCriticalErrors(t *testing.T) {
	client := fake.NewSimpleClientset()
	client.PrependReactor("list", "networkpolicies", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, context.DeadlineExceeded
	})
	if _, err := GetNetworkPolicyList(client, common.NewNamespaceQuery(nil), dataselect.NoDataSelect); err == nil {
		t.Fatal("GetNetworkPolicyList() ignored a critical Kubernetes error")
	}
}
