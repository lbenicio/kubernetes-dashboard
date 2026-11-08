// Copyright 2026 The Kubernetes Authors.
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

package sidecar

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	apimachinerytypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"

	metricapi "k8s.io/dashboard/api/pkg/integration/metric/api"
	"k8s.io/dashboard/types"
)

type recordingMetricSidecar struct {
	path string
	body []byte
	err  error
}

func (s *recordingMetricSidecar) Get(path string) RequestInterface {
	s.path = path
	return metricSidecarRequest{body: s.body, err: s.err}
}

func (*recordingMetricSidecar) HealthCheck() error { return nil }

type metricSidecarRequest struct {
	body []byte
	err  error
}

func (r metricSidecarRequest) DoRaw(context.Context) ([]byte, error) { return r.body, r.err }
func (metricSidecarRequest) AbsPath(...string) *rest.Request         { return nil }

func TestIthResourceDownloadHandlesRequestAndDecodeResults(t *testing.T) {
	requestErr := errors.New("sidecar unavailable")
	metricPoint := metricapi.MetricPoint{Timestamp: time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC), Value: 42}
	validBody, err := json.Marshal(metricapi.SidecarMetricResultList{Items: []metricapi.SidecarMetric{{MetricName: "cpu", MetricPoints: []metricapi.MetricPoint{metricPoint}}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name       string
		body       []byte
		requestErr error
		wantMetric bool
		wantError  bool
	}{
		{name: "request error", requestErr: requestErr, wantError: true},
		{name: "invalid JSON", body: []byte("{"), wantError: true},
		{name: "no matching samples", body: []byte(`{"items":[]}`)},
		{name: "sample returned", body: validBody, wantMetric: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			restClient := &recordingMetricSidecar{body: test.body, err: test.requestErr}
			client := sidecarClient{client: restClient}
			selector := sidecarSelector{
				Path: "nodes/", Resources: []string{"node-a"}, TargetResourceType: types.ResourceKindNode,
				Label: metricapi.Label{types.ResourceKindNode: {apimachinerytypes.UID("node-uid")}},
			}
			metric, err := client.ithResourceDownload(selector, "cpu", 0).GetMetric()
			if (err != nil) != test.wantError || (metric != nil) != test.wantMetric {
				t.Fatalf("ithResourceDownload() = %#v, %v; want metric=%t error=%t", metric, err, test.wantMetric, test.wantError)
			}
			if restClient.path != "/api/v1/dashboard/nodes/node-a/metrics/cpu" {
				t.Errorf("sidecar request path = %q", restClient.path)
			}
			if test.wantMetric && (metric.MetricName != "cpu" || metric.Label[types.ResourceKindNode][0] != "node-uid" || metric.MetricPoints[0].Value != 42) {
				t.Errorf("decoded metric = %#v", metric)
			}
		})
	}
}

func TestAllInOneDownloadMatchesResourcesAndReportsMissingResults(t *testing.T) {
	metricPoint := metricapi.MetricPoint{Timestamp: time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC), Value: 7}
	response := func(items ...metricapi.SidecarMetric) []byte {
		body, err := json.Marshal(metricapi.SidecarMetricResultList{Items: items})
		if err != nil {
			t.Fatalf("marshal sidecar fixture: %v", err)
		}
		return body
	}
	selector := sidecarSelector{
		Path: "namespaces/default/pod-list/", Resources: []string{"pod-a", "pod-b"},
		TargetResourceType: types.ResourceKindPod,
		Label:              metricapi.Label{types.ResourceKindPod: {apimachinerytypes.UID("uid-a"), apimachinerytypes.UID("uid-b")}},
	}

	t.Run("preserves requested order", func(t *testing.T) {
		restClient := &recordingMetricSidecar{body: response(
			metricapi.SidecarMetric{MetricName: "cpu", UIDs: []string{"pod-b"}, MetricPoints: []metricapi.MetricPoint{metricPoint}},
			metricapi.SidecarMetric{MetricName: "cpu", UIDs: []string{"pod-a"}, MetricPoints: []metricapi.MetricPoint{metricPoint}},
		)}
		metrics, err := (sidecarClient{client: restClient}).allInOneDownload(selector, "cpu").GetMetrics()
		if err != nil || len(metrics) != 2 {
			t.Fatalf("allInOneDownload() = %#v, %v", metrics, err)
		}
		if metrics[0].Label[types.ResourceKindPod][0] != "uid-a" || metrics[1].Label[types.ResourceKindPod][0] != "uid-b" {
			t.Errorf("mapped metrics = %#v; want selector order uid-a, uid-b", metrics)
		}
		if restClient.path != "/api/v1/dashboard/namespaces/default/pod-list/pod-a,pod-b/metrics/cpu" {
			t.Errorf("sidecar request path = %q", restClient.path)
		}
	})

	t.Run("missing resource result returns an error", func(t *testing.T) {
		restClient := &recordingMetricSidecar{body: response(metricapi.SidecarMetric{MetricName: "cpu", UIDs: []string{"pod-a"}, MetricPoints: []metricapi.MetricPoint{metricPoint}})}
		promises := (sidecarClient{client: restClient}).allInOneDownload(selector, "cpu")
		if metric, err := promises[0].GetMetric(); err != nil || metric == nil {
			t.Fatalf("allInOneDownload(pod-a) = %#v, %v; want its metric", metric, err)
		}
		if metric, err := promises[1].GetMetric(); err == nil || metric != nil {
			t.Fatalf("allInOneDownload(pod-b) = %#v, %v; want missing-resource error", metric, err)
		}
	})

	t.Run("request error propagates", func(t *testing.T) {
		requestErr := errors.New("sidecar unavailable")
		restClient := &recordingMetricSidecar{err: requestErr}
		promises := (sidecarClient{client: restClient}).allInOneDownload(selector, "cpu")
		for i, promise := range promises {
			if _, err := promise.GetMetric(); !errors.Is(err, requestErr) {
				t.Errorf("allInOneDownload() promise %d error = %v; want %v", i, err, requestErr)
			}
		}
	})

	t.Run("empty selector makes no request", func(t *testing.T) {
		restClient := &recordingMetricSidecar{}
		metrics, err := (sidecarClient{client: restClient}).allInOneDownload(sidecarSelector{}, "cpu").GetMetrics()
		if err != nil || len(metrics) != 0 || restClient.path != "" {
			t.Errorf("allInOneDownload(empty) = %#v, %v, path %q", metrics, err, restClient.path)
		}
	})
}
