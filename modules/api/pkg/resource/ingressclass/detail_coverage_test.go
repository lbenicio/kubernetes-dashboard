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
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
package ingressclass

import (
	"testing"
	"time"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestIngressClassCellPropertiesAndConversions(t *testing.T) {
	created := metav1.NewTime(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	class := networkingv1.IngressClass{ObjectMeta: metav1.ObjectMeta{Name: "nginx", CreationTimestamp: created}}
	cell := IngressClassCell(class)
	for _, tc := range []struct {
		property dataselect.PropertyName
		want     dataselect.ComparableValue
	}{
		{dataselect.NameProperty, dataselect.StdComparableString("nginx")},
		{dataselect.NamespaceProperty, dataselect.StdComparableString("")},
		{dataselect.CreationTimestampProperty, dataselect.StdComparableTime(created.Time)},
	} {
		if got := cell.GetProperty(tc.property); got != tc.want {
			t.Errorf("GetProperty(%s) = %v; want %v", tc.property, got, tc.want)
		}
	}
	if cell.GetProperty(dataselect.PropertyName("unsupported")) != nil {
		t.Error("unsupported property should return nil")
	}
	if got := fromCells(toCells([]networkingv1.IngressClass{class})); len(got) != 1 || got[0].Name != class.Name {
		t.Errorf("cell conversion = %#v", got)
	}
}

func TestGetIngressClassAndParameters(t *testing.T) {
	apiGroup, namespace, scope := "example.test", "ingress-system", "Namespace"
	class := &networkingv1.IngressClass{ObjectMeta: metav1.ObjectMeta{Name: "nginx"}, Spec: networkingv1.IngressClassSpec{Parameters: &networkingv1.IngressClassParametersReference{APIGroup: &apiGroup, Kind: "ConfigMap", Name: "controller-config", Namespace: &namespace, Scope: &scope}}}
	client := fake.NewSimpleClientset(class)
	detail, err := GetIngressClass(client, "nginx")
	if err != nil || detail.ObjectMeta.Name != "nginx" || detail.Parameters["Kind"] != "ConfigMap" || detail.Parameters["ApiGroup"] != apiGroup || detail.Parameters["Name"] != "controller-config" || detail.Parameters["Namespace"] != namespace || detail.Parameters["Scope"] != scope {
		t.Fatalf("GetIngressClass() = %#v, %v", detail, err)
	}
	if _, err := GetIngressClass(client, "missing"); err == nil {
		t.Fatal("GetIngressClass() succeeded for missing class")
	}
}
