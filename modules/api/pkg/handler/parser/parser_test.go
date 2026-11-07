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

package parser

import (
	"net/http/httptest"
	"testing"

	"github.com/emicklei/go-restful/v3"
	"k8s.io/dashboard/api/pkg/integration/metric/api"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestParsePaginationPathParameter(t *testing.T) {
	cases := []struct {
		name, query string
		items, page int
	}{
		{name: "converts one-based page", query: "?itemsPerPage=25&page=3", items: 25, page: 2},
		{name: "invalid size disables pagination", query: "?itemsPerPage=no&page=3", items: -1, page: -1},
		{name: "invalid page disables pagination", query: "?itemsPerPage=25&page=0x", items: -1, page: -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := restfulRequest(tc.query)
			got := parsePaginationPathParameter(request)
			if got.ItemsPerPage != tc.items || got.Page != tc.page {
				t.Fatalf("pagination = %#v, want items=%d page=%d", got, tc.items, tc.page)
			}
		})
	}
}

func TestParseFilterSortAndMetricsParameters(t *testing.T) {
	request := restfulRequest("?filterBy=name,web,namespace,default&sortBy=a,name,d,creationTimestamp&metricNames=cpu/usage_rate,memory/usage&aggregations=sum,max")
	filter := parseFilterPathParameter(request)
	if len(filter.FilterByList) != 2 || filter.FilterByList[0].Property != "name" || filter.FilterByList[0].Value != dataselect.StdComparableString("web") {
		t.Fatalf("filter = %#v", filter)
	}
	sort := parseSortPathParameter(request)
	if len(sort.SortByList) != 2 || !sort.SortByList[0].Ascending || sort.SortByList[0].Property != "name" || sort.SortByList[1].Ascending {
		t.Fatalf("sort = %#v", sort)
	}
	metrics := parseMetricPathParameter(request)
	if len(metrics.MetricNames) != 2 || metrics.MetricNames[0] != api.CpuUsage || metrics.MetricNames[1] != api.MemoryUsage {
		t.Fatalf("metric names = %#v", metrics.MetricNames)
	}
	if len(metrics.Aggregations) != 2 || metrics.Aggregations[0] != api.SumAggregation || metrics.Aggregations[1] != api.MaxAggregation {
		t.Fatalf("metric aggregations = %#v", metrics.Aggregations)
	}
}

func TestParseEmptyAndMalformedOptionalParameters(t *testing.T) {
	request := restfulRequest("")
	metrics := parseMetricPathParameter(request)
	if metrics.MetricNames != nil || len(metrics.Aggregations) != 0 {
		t.Fatalf("empty metrics query = %#v", metrics)
	}
	filter := parseFilterPathParameter(request)
	if len(filter.FilterByList) != 0 {
		t.Fatalf("empty filter query = %#v", filter)
	}
	sort := parseSortPathParameter(request)
	if len(sort.SortByList) != 0 {
		t.Fatalf("empty sort query = %#v", sort)
	}

	request = restfulRequest("?filterBy=name&sortBy=up,name&aggregations=sum,avg")
	if got := parseFilterPathParameter(request); len(got.FilterByList) != 0 {
		t.Errorf("odd filter query = %#v", got)
	}
	if got := parseSortPathParameter(request); len(got.SortByList) != 0 {
		t.Errorf("invalid sort query = %#v", got)
	}
	if got := parseMetricPathParameter(request); len(got.Aggregations) != 2 {
		t.Errorf("aggregation-only query = %#v", got)
	}
}

func TestParseDataSelectPathParameterCombinesQueries(t *testing.T) {
	got := ParseDataSelectPathParameter(restfulRequest("?itemsPerPage=5&page=2&filterBy=name,api&sortBy=d,name&metricNames=cpu/usage_rate"))
	if got.PaginationQuery.ItemsPerPage != 5 || got.PaginationQuery.Page != 1 || len(got.FilterQuery.FilterByList) != 1 || len(got.SortQuery.SortByList) != 1 || len(got.MetricQuery.MetricNames) != 1 {
		t.Fatalf("data select query = %#v", got)
	}
}

func restfulRequest(query string) *restful.Request {
	request := httptest.NewRequest("GET", "/api/v1/pod"+query, nil)
	return &restful.Request{Request: request}
}
