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
package horizontalpodautoscaler

import (
	"testing"
	"time"

	autoscaling "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestHorizontalPodAutoscalerCellPropertiesAndConversions(t *testing.T) {
	created := metav1.NewTime(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	hpa := autoscaling.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", CreationTimestamp: created}}
	cell := HorizontalPodAutoscalerCell(hpa)
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
	if got := fromCells(toCells([]autoscaling.HorizontalPodAutoscaler{hpa})); len(got) != 1 || got[0].Name != hpa.Name {
		t.Errorf("cell conversion = %#v", got)
	}
}
