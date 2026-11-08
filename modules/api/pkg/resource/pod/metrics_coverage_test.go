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

package pod

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	integrationapi "k8s.io/dashboard/api/pkg/integration/api"
	metricapi "k8s.io/dashboard/api/pkg/integration/metric/api"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

type invalidPodMetricLabelsClient struct{}

func (invalidPodMetricLabelsClient) DownloadMetric(_ []metricapi.ResourceSelector, name string, _ *metricapi.CachedResources) metricapi.MetricPromises {
	promises := metricapi.NewMetricPromises(1)
	promises.PutMetrics([]metricapi.Metric{{MetricName: name}}, nil)
	return promises
}

func (invalidPodMetricLabelsClient) DownloadMetrics([]metricapi.ResourceSelector, []string, *metricapi.CachedResources) metricapi.MetricPromises {
	return nil
}

func (invalidPodMetricLabelsClient) AggregateMetrics(promises metricapi.MetricPromises, _ string, _ metricapi.AggregationModes) metricapi.MetricPromises {
	return promises
}

func (invalidPodMetricLabelsClient) ID() integrationapi.IntegrationID { return "invalid-labels" }
func (invalidPodMetricLabelsClient) HealthCheck() error               { return nil }

func TestGetMetricsPerPodSkipsMetricsWithoutPodUIDLabels(t *testing.T) {
	metrics, err := getMetricsPerPod([]corev1.Pod{{ObjectMeta: metav1.ObjectMeta{Name: "web", UID: types.UID("pod-uid")}}},
		invalidPodMetricLabelsClient{}, dataselect.NoDataSelect)
	if err != nil {
		t.Fatalf("getMetricsPerPod() error = %v", err)
	}
	if metrics == nil || len(metrics.MetricsMap) != 0 {
		t.Fatalf("getMetricsPerPod() = %#v; want an empty metrics map", metrics)
	}
}
