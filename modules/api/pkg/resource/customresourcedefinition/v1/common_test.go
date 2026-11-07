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
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

package v1

import (
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/dashboard/api/pkg/resource/customresourcedefinition/types"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
	commontypes "k8s.io/dashboard/types"
)

func TestCustomResourceDefinitionCellPropertiesAndConversions(t *testing.T) {
	timestamp := metav1.NewTime(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	crds := []apiextensions.CustomResourceDefinition{{ObjectMeta: metav1.ObjectMeta{Name: "widgets.example.test", CreationTimestamp: timestamp}}}
	cells := toCells(crds)
	if got := cells[0].(CustomResourceDefinitionCell).GetProperty(dataselect.NameProperty); got != dataselect.StdComparableString("widgets.example.test") {
		t.Errorf("name property = %v", got)
	}
	if got := cells[0].(CustomResourceDefinitionCell).GetProperty(dataselect.CreationTimestampProperty); got != dataselect.StdComparableTime(timestamp.Time) {
		t.Errorf("creation property = %v", got)
	}
	if got := cells[0].(CustomResourceDefinitionCell).GetProperty(dataselect.NamespaceProperty); got != nil {
		t.Errorf("unsupported property = %v; want nil", got)
	}
	if got := fromCells(cells); !reflect.DeepEqual(got, crds) {
		t.Errorf("fromCells() = %#v; want %#v", got, crds)
	}
}

func TestCustomResourceObjectCellPropertiesAndConversions(t *testing.T) {
	creationTime := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	objects := []types.CustomResourceObject{{ObjectMeta: commontypes.ObjectMeta{Name: "widget", Namespace: "default", CreationTimestamp: metav1.NewTime(creationTime)}}}
	cells := toObjectCells(objects)
	cell := cells[0].(CustomResourceObjectCell)
	if got := cell.GetProperty(dataselect.NameProperty); got != dataselect.StdComparableString("widget") {
		t.Errorf("name property = %v", got)
	}
	if got := cell.GetProperty(dataselect.NamespaceProperty); got != dataselect.StdComparableString("default") {
		t.Errorf("namespace property = %v", got)
	}
	if got := cell.GetProperty(dataselect.CreationTimestampProperty); got != dataselect.StdComparableTime(creationTime) {
		t.Errorf("creation property = %v", got)
	}
	if got := cell.GetProperty(dataselect.PropertyName("kind")); got != nil {
		t.Errorf("unsupported property = %v; want nil", got)
	}
	if got := fromObjectCells(cells); !reflect.DeepEqual(got, objects) {
		t.Errorf("fromObjectCells() = %#v; want %#v", got, objects)
	}
}

func TestCRDVersionAndConditionsHelpers(t *testing.T) {
	transition := metav1.NewTime(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	crd := &apiextensions.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "widgets.example.test"},
		Spec: apiextensions.CustomResourceDefinitionSpec{
			Group: "example.test",
			Versions: []apiextensions.CustomResourceDefinitionVersion{
				{Name: "v1", Served: true},
				{Name: "v1beta1", Served: false},
			},
		},
		Status: apiextensions.CustomResourceDefinitionStatus{Conditions: []apiextensions.CustomResourceDefinitionCondition{{
			Type: apiextensions.Established, Status: apiextensions.ConditionTrue,
			LastTransitionTime: transition, Reason: "Accepted", Message: "ready",
		}}},
	}
	if got := getCustomResourceDefinitionGroupVersion(crd); got != (schema.GroupVersion{Group: "example.test", Version: "v1"}) {
		t.Errorf("group version = %v", got)
	}
	if got := getCRDConditions(crd); len(got) != 1 || got[0].Type != string(apiextensions.Established) || got[0].Status != corev1.ConditionTrue || got[0].Reason != "Accepted" || got[0].Message != "ready" {
		t.Errorf("conditions = %#v", got)
	}
	if !isServed(*crd) {
		t.Error("isServed() = false for served version")
	}
	filtered := removeNonServedVersions(*crd)
	if len(filtered.Spec.Versions) != 1 || filtered.Spec.Versions[0].Name != "v1" {
		t.Errorf("served versions = %#v; want only v1", filtered.Spec.Versions)
	}
	if isServed(apiextensions.CustomResourceDefinition{Spec: apiextensions.CustomResourceDefinitionSpec{Versions: []apiextensions.CustomResourceDefinitionVersion{{Name: "v1", Served: false}}}}) {
		t.Error("isServed() = true when every version is unserved")
	}
}
