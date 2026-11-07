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

package poddisruptionbudget

import (
	"errors"
	"testing"
	"time"

	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"

	"k8s.io/dashboard/api/pkg/resource/common"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestCellPropertiesAndConversions(t *testing.T) {
	created := metav1.NewTime(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	budget := policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", CreationTimestamp: created}}
	cell := PodDisruptionBudgetCell(budget)
	for _, tc := range []struct {
		property dataselect.PropertyName
		want     dataselect.ComparableValue
	}{
		{dataselect.NameProperty, dataselect.StdComparableString("web")},
		{dataselect.NamespaceProperty, dataselect.StdComparableString("default")},
		{dataselect.CreationTimestampProperty, dataselect.StdComparableTime(created.Time)},
	} {
		if got := cell.GetProperty(tc.property); got != tc.want {
			t.Errorf("GetProperty(%s) = %v; want %v", tc.property, got, tc.want)
		}
	}
	if cell.GetProperty(dataselect.PropertyName("unsupported")) != nil {
		t.Error("unsupported property should return nil")
	}
	if got := fromCells(toCells([]policyv1.PodDisruptionBudget{budget})); len(got) != 1 || got[0].Name != budget.Name {
		t.Errorf("cell conversion = %#v", got)
	}
}

func TestClientBackedListAndDetail(t *testing.T) {
	budget := &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"}, Status: policyv1.PodDisruptionBudgetStatus{CurrentHealthy: 2, DesiredHealthy: 2}}
	client := fake.NewSimpleClientset(budget)
	list, err := List(client, common.NewSameNamespaceQuery("default"), dataselect.NoDataSelect)
	if err != nil || list.ListMeta.TotalItems != 1 || len(list.Items) != 1 || list.Items[0].ObjectMeta.Name != "web" {
		t.Fatalf("List() = %#v, %v", list, err)
	}
	detail, err := Get(client, "default", "web")
	if err != nil || detail.ObjectMeta.Name != "web" || detail.CurrentHealthy != 2 {
		t.Fatalf("Get() = %#v, %v", detail, err)
	}
	if _, err := Get(client, "default", "missing"); err == nil {
		t.Fatal("Get() succeeded for missing budget")
	}
}

func TestListReturnsClientError(t *testing.T) {
	client := fake.NewSimpleClientset()
	client.PrependReactor("list", "poddisruptionbudgets", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("list failed")
	})
	if _, err := List(client, common.NewNamespaceQuery(nil), dataselect.NoDataSelect); err == nil || err.Error() != "list failed" {
		t.Fatalf("List() error = %v; want list failed", err)
	}
}
