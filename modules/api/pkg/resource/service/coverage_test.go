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

package service

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestServiceCellPropertiesAndConversions(t *testing.T) {
	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "team-a"},
		Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeNodePort},
	}
	cell := ServiceCell(service)

	for name, want := range map[dataselect.PropertyName]dataselect.ComparableValue{
		dataselect.NameProperty:      dataselect.StdComparableString("web"),
		dataselect.NamespaceProperty: dataselect.StdComparableString("team-a"),
		dataselect.TypeProperty:      dataselect.StdComparableString(string(corev1.ServiceTypeNodePort)),
	} {
		if got := cell.GetProperty(name); got != want {
			t.Errorf("GetProperty(%q) = %v, want %v", name, got, want)
		}
	}
	if got := cell.GetProperty(dataselect.CreationTimestampProperty); got == nil {
		t.Error("creation timestamp property is nil")
	}
	if got := cell.GetProperty("unsupported"); got != nil {
		t.Errorf("unsupported property = %v, want nil", got)
	}
	if got := fromCells(toCells([]corev1.Service{service})); len(got) != 1 || got[0].Name != "web" {
		t.Errorf("service cell conversion = %#v", got)
	}
}

func TestGetServiceIngressListFiltersByService(t *testing.T) {
	serviceBackend := func(name string) networkingv1.IngressBackend {
		return networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: name}}
	}
	client := fake.NewClientset(&networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "team-a"},
		Spec: networkingv1.IngressSpec{
			DefaultBackend: &networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: "web-svc"}},
			Rules: []networkingv1.IngressRule{{IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{Paths: []networkingv1.HTTPIngressPath{{Backend: serviceBackend("other-svc")}}},
			}}},
		},
	})

	got, err := GetServiceIngressList(client, dataselect.NoDataSelect, "team-a", "web-svc")
	if err != nil {
		t.Fatalf("GetServiceIngressList() error = %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Name != "web" {
		t.Fatalf("GetServiceIngressList() = %#v, want matching ingress", got)
	}

	got, err = GetServiceIngressList(client, dataselect.NoDataSelect, "team-a", "missing")
	if err != nil {
		t.Fatalf("GetServiceIngressList() with no match error = %v", err)
	}
	if len(got.Items) != 0 {
		t.Errorf("GetServiceIngressList() returned %d unrelated ingresses", len(got.Items))
	}
}
