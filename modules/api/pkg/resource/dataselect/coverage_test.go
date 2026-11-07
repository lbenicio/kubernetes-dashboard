// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package dataselect

import (
	"testing"

	integrationapi "k8s.io/dashboard/api/pkg/integration/api"
	metricapi "k8s.io/dashboard/api/pkg/integration/metric/api"
)

type metricCell struct{ TestDataCell }

func (in metricCell) GetResourceSelector() *metricapi.ResourceSelector {
	return &metricapi.ResourceSelector{ResourceName: in.Name}
}

type recordingMetricClient struct {
	downloaded []string
	aggregated []string
	selectors  [][]metricapi.ResourceSelector
}

func (in *recordingMetricClient) DownloadMetric(selectors []metricapi.ResourceSelector, name string, _ *metricapi.CachedResources) metricapi.MetricPromises {
	in.downloaded = append(in.downloaded, name)
	in.selectors = append(in.selectors, selectors)
	return metricapi.NewMetricPromises(1)
}

func (in *recordingMetricClient) DownloadMetrics(_ []metricapi.ResourceSelector, _ []string, _ *metricapi.CachedResources) metricapi.MetricPromises {
	return nil
}

func (in *recordingMetricClient) AggregateMetrics(promises metricapi.MetricPromises, name string, _ metricapi.AggregationModes) metricapi.MetricPromises {
	in.aggregated = append(in.aggregated, name)
	return promises
}

func (*recordingMetricClient) ID() integrationapi.IntegrationID { return "coverage" }
func (*recordingMetricClient) HealthCheck() error               { return nil }

func TestFilterAndDataSelectionHelpers(t *testing.T) {
	cells := getDataCellList()
	selector := DataSelector{
		GenericDataList: cells,
		DataSelectQuery: NewDataSelectQuery(NoPagination, NoSort, NewFilterQuery([]string{"name", "ab"}), NoMetrics),
	}
	filtered := selector.Filter().GenericDataList
	if len(filtered) != 3 {
		t.Fatalf("Filter() retained %d cells; want 3", len(filtered))
	}

	withFilter, total := GenericDataSelectWithFilter(cells, NewDataSelectQuery(NewPaginationQuery(2, 0), NoSort, NewFilterQuery([]string{"name", "ab"}), NoMetrics))
	if total != 3 || len(withFilter) != 2 {
		t.Fatalf("GenericDataSelectWithFilter() returned %d cells and total %d; want 2 and 3", len(withFilter), total)
	}
	if got := GenericDataSelect(cells, NewDataSelectQuery(NewPaginationQuery(1, 0), NoSort, NoFilter, NoMetrics)); len(got) != 1 {
		t.Fatalf("GenericDataSelect() returned %d cells; want 1", len(got))
	}
}

func TestMetricSelectionHelpers(t *testing.T) {
	query := NewDataSelectQuery(NewPaginationQuery(1, 0), NoSort, NoFilter, NewMetricQuery([]string{"cpu", "memory"}, nil))
	cells := []DataCell{metricCell{TestDataCell{Name: "pod-a", Id: 1}}, TestDataCell{Name: "non-metric", Id: 2}}
	client := &recordingMetricClient{}

	selected, promises := GenericDataSelectWithMetrics(cells, query, metricapi.NoResourceCache, client)
	if len(selected) != 1 || len(promises) != 2 {
		t.Fatalf("GenericDataSelectWithMetrics() returned %d cells and %d promises", len(selected), len(promises))
	}
	if len(client.downloaded) != 2 || len(client.aggregated) != 2 || client.downloaded[0] != "cpu" || client.aggregated[1] != "memory" {
		t.Fatalf("metric calls: downloads=%v aggregates=%v", client.downloaded, client.aggregated)
	}
	if len(client.selectors[0]) != 2 || client.selectors[0][0].ResourceName != "pod-a" || client.selectors[0][1].ResourceName != "" {
		t.Fatalf("metric selectors = %#v", client.selectors[0])
	}

	filtered, filteredPromises, total := GenericDataSelectWithFilterAndMetrics(cells, query, metricapi.NoResourceCache, client)
	if len(filtered) != 1 || len(filteredPromises) != 2 || total != 2 {
		t.Fatalf("GenericDataSelectWithFilterAndMetrics() = %d cells, %d promises, total %d", len(filtered), len(filteredPromises), total)
	}
	if got := PodListMetrics(cells, query, client); len(got) != 2 {
		t.Fatalf("PodListMetrics() returned %d promises; want 2", len(got))
	}
}

func TestMetricSelectionSkipsNilClientAndMissingNames(t *testing.T) {
	selector := DataSelector{GenericDataList: getDataCellList(), DataSelectQuery: NewDataSelectQuery(NoPagination, NoSort, NoFilter, NoMetrics)}
	if got := selector.GetMetrics(nil).MetricsPromises; len(got) != 0 {
		t.Fatalf("GetMetrics(nil) returned %d promises", len(got))
	}
	if got := selector.GetCumulativeMetrics(nil).CumulativeMetricsPromises; len(got) != 0 {
		t.Fatalf("GetCumulativeMetrics(nil) returned %d promises", len(got))
	}
	client := &recordingMetricClient{}
	if got := selector.GetMetrics(client).MetricsPromises; len(got) != 0 {
		t.Fatalf("GetMetrics() without metric names returned %d promises", len(got))
	}
}
