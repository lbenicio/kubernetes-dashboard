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

package api

import (
	"errors"
	"strings"
	"testing"

	dashboardtypes "k8s.io/dashboard/types"
)

func TestMetricLabelsMergeWithoutDuplicateUIDs(t *testing.T) {
	base := Label{dashboardtypes.ResourceKindPod: {"pod-a"}}
	merged := base.AddMetricLabel(Label{
		dashboardtypes.ResourceKindPod:  {"pod-a", "pod-b", "pod-b"},
		dashboardtypes.ResourceKindNode: {"node-a"},
	})
	if len(merged[dashboardtypes.ResourceKindPod]) != 2 || merged[dashboardtypes.ResourceKindPod][1] != "pod-b" {
		t.Fatalf("merged pod labels = %v", merged[dashboardtypes.ResourceKindPod])
	}
	if len(merged[dashboardtypes.ResourceKindNode]) != 1 || merged[dashboardtypes.ResourceKindNode][0] != "node-a" {
		t.Fatalf("merged node labels = %v", merged[dashboardtypes.ResourceKindNode])
	}
	if base[dashboardtypes.ResourceKindPod][0] != "pod-a" {
		t.Fatal("merging labels changed existing identity")
	}

	if got := base.AddMetricLabel(nil); len(got[dashboardtypes.ResourceKindPod]) != 2 {
		t.Fatalf("nil label merge changed values: %v", got)
	}
}

func TestMetricPointsAndString(t *testing.T) {
	point := MetricPoint{Value: 42}
	sidecar := &SidecarMetric{MetricName: "cpu"}
	metric := &Metric{MetricName: "cpu", Aggregate: SumAggregation}
	if got := sidecar.AddMetricPoint(point); len(got) != 1 || got[0].Value != 42 {
		t.Fatalf("SidecarMetric.AddMetricPoint() = %v", got)
	}
	if got := metric.AddMetricPoint(point); len(got) != 1 || got[0].Value != 42 {
		t.Fatalf("Metric.AddMetricPoint() = %v", got)
	}
	for _, want := range []string{"DataPoints:", "MetricPoints:", "MetricName: cpu", "Aggregate: sum"} {
		if !strings.Contains(metric.String(), want) {
			t.Errorf("Metric.String() missing %q: %s", want, metric.String())
		}
	}
}

func TestMetricPromisesDeliverResultsAndSkipFailures(t *testing.T) {
	promises := NewMetricPromises(2)
	want := []Metric{{MetricName: "cpu"}, {MetricName: "memory"}}
	promises.PutMetrics(want, nil)
	got, err := promises.GetMetrics()
	if err != nil || len(got) != 2 || got[0].MetricName != "cpu" || got[1].MetricName != "memory" {
		t.Fatalf("GetMetrics() = %#v, %v", got, err)
	}

	failed := NewMetricPromises(1)
	failed.PutMetrics(nil, errors.New("metrics unavailable"))
	got, err = failed.GetMetrics()
	if err != nil || len(got) != 0 {
		t.Fatalf("GetMetrics() with rejected promise = %#v, %v", got, err)
	}

	nilMetric := NewMetricPromise()
	nilMetric.Metric <- nil
	nilMetric.Error <- nil
	got, err = MetricPromises{nilMetric}.GetMetrics()
	if err != nil || len(got) != 0 {
		t.Fatalf("GetMetrics() with nil metric = %#v, %v", got, err)
	}

	var empty MetricPromises
	got, err = empty.GetMetrics()
	if err != nil || len(got) != 0 {
		t.Fatalf("empty promises = %#v, %v", got, err)
	}
}

func TestAggregationHelpers(t *testing.T) {
	values := []int64{5, -2, 11, 4}
	if got := SumAggregate(values); got != 18 {
		t.Errorf("SumAggregate() = %d, want 18", got)
	}
	if got := MaxAggregate(values); got != 11 {
		t.Errorf("MaxAggregate() = %d, want 11", got)
	}
	if got := MinAggregate(values); got != -2 {
		t.Errorf("MinAggregate() = %d, want -2", got)
	}
	if got := MaxAggregate([]int64{7, 7}); got != 7 {
		t.Errorf("MaxAggregate(equal) = %d, want 7", got)
	}
}
