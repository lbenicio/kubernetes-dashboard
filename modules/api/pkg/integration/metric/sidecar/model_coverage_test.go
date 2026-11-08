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

package sidecar

import (
	"math"
	"testing"
	"time"

	metricapi "k8s.io/dashboard/api/pkg/integration/metric/api"
)

func TestDataPointsFromMetricJSONFormatClampsOverflowedValues(t *testing.T) {
	point := metricapi.MetricPoint{Timestamp: time.Unix(10, 0), Value: uint64(math.MaxInt64) + 1}
	got := DataPointsFromMetricJSONFormat([]metricapi.MetricPoint{point})
	if len(got) != 1 || got[0].X != 10 || got[0].Y != 0 {
		t.Fatalf("converted overflowed metric = %#v; want timestamp 10 and clamped value 0", got)
	}
}
