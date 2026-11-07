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

package provider

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"k8s.io/apimachinery/pkg/types"
	_ "modernc.org/sqlite"

	"k8s.io/dashboard/metrics-scraper/pkg/database"
)

func TestGetNodeMetricsAggregatesPointsAndFiltersResources(t *testing.T) {
	db := newDashboardTestDB(t)
	insertMetric(t, db, `INSERT INTO nodes(uid, name, cpu, memory, time) VALUES (?, ?, ?, ?, ?)`, "node-uid", "node-a", "25", "100", "2026-11-07T00:00:00Z")
	insertMetric(t, db, `INSERT INTO nodes(uid, name, cpu, memory, time) VALUES (?, ?, ?, ?, ?)`, "node-uid", "node-a", "30", "125", "2026-11-07T00:01:00Z")
	insertMetric(t, db, `INSERT INTO nodes(uid, name, cpu, memory, time) VALUES (?, ?, ?, ?, ?)`, "other-uid", "node-b", "99", "200", "2026-11-07T00:01:00Z")

	got, err := getNodeMetrics(db, "cpu", ResourceSelector{ResourceName: "node-a", UID: types.UID("node-uid")})
	if err != nil {
		t.Fatalf("getNodeMetrics() error = %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("got %d node metrics, want 1", len(got.Items))
	}
	metric := got.Items[0]
	if metric.MetricName != "cpu" || len(metric.MetricPoints) != 2 || metric.UIDs[0] != "node-a" {
		t.Fatalf("node metric = %#v", metric)
	}
	if metric.MetricPoints[0].Value != 25 || metric.MetricPoints[1].Value != 30 {
		t.Fatalf("node metric points = %#v", metric.MetricPoints)
	}

	memory, err := getNodeMetrics(db, "memory/usage", ResourceSelector{ResourceName: "node-b"})
	if err != nil || len(memory.Items) != 1 || memory.Items[0].MetricPoints[0].Value != 200 {
		t.Fatalf("getNodeMetrics(memory) = %#v, %v", memory, err)
	}
}

func TestGetPodMetricsUsesDefaultNamespaceAndNameList(t *testing.T) {
	db := newDashboardTestDB(t)
	insertMetric(t, db, `INSERT INTO pods(uid, name, namespace, container, cpu, memory, time) VALUES (?, ?, ?, ?, ?, ?, ?)`, "default-uid", "pod-a", "default", "container-a", "10", "100", "2026-11-07T00:00:00Z")
	insertMetric(t, db, `INSERT INTO pods(uid, name, namespace, container, cpu, memory, time) VALUES (?, ?, ?, ?, ?, ?, ?)`, "default-uid", "pod-a", "default", "container-b", "15", "200", "2026-11-07T00:00:00Z")
	insertMetric(t, db, `INSERT INTO pods(uid, name, namespace, container, cpu, memory, time) VALUES (?, ?, ?, ?, ?, ?, ?)`, "other-uid", "pod-b", "other", "container-a", "20", "300", "2026-11-07T00:00:00Z")

	got, err := getPodMetrics(db, "cpu", ResourceSelector{ResourceName: "pod-a,pod-b"})
	if err != nil {
		t.Fatalf("getPodMetrics() error = %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].MetricName != "cpu" || len(got.Items[0].MetricPoints) != 1 {
		t.Fatalf("pod metrics = %#v", got)
	}
	if got.Items[0].MetricPoints[0].Value != 25 {
		t.Fatalf("aggregated pod CPU = %d, want 25", got.Items[0].MetricPoints[0].Value)
	}

	other, err := getPodMetrics(db, "memory/usage", ResourceSelector{Namespace: "other", ResourceName: "pod-b"})
	if err != nil || len(other.Items) != 1 || other.Items[0].MetricPoints[0].Value != 300 {
		t.Fatalf("getPodMetrics(memory) = %#v, %v", other, err)
	}
}

func TestMetricQueriesReportDatabaseAndRowErrors(t *testing.T) {
	db := newDashboardTestDB(t)
	if _, err := getNodeMetrics(db, "cpu", ResourceSelector{ResourceName: "missing"}); err != nil {
		t.Fatalf("query without matching rows should succeed: %v", err)
	}
	if _, err := getPodMetrics(db, "cpu", ResourceSelector{Namespace: "default", ResourceName: "missing"}); err != nil {
		t.Fatalf("pod query without matching rows should succeed: %v", err)
	}

	insertMetric(t, db, `INSERT INTO nodes(uid, name, cpu, memory, time) VALUES (?, ?, ?, ?, ?)`, "node-uid", "bad-time", "25", "100", "invalid-time")
	if _, err := getNodeMetrics(db, "cpu", ResourceSelector{ResourceName: "bad-time"}); err == nil {
		t.Fatal("getNodeMetrics() accepted an invalid timestamp")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := getPodMetrics(db, "cpu", ResourceSelector{}); err == nil {
		t.Fatal("getPodMetrics() accepted a closed database")
	}
}

func TestDashboardRouterAndHandlers(t *testing.T) {
	db := newDashboardTestDB(t)
	insertMetric(t, db, `INSERT INTO nodes(uid, name, cpu, memory, time) VALUES (?, ?, ?, ?, ?)`, "node-uid", "node-a", "25", "100", "2026-11-07T00:00:00Z")
	insertMetric(t, db, `INSERT INTO pods(uid, name, namespace, container, cpu, memory, time) VALUES (?, ?, ?, ?, ?, ?, ?)`, "pod-uid", "pod-a", "default", "container-a", "10", "100", "2026-11-07T00:00:00Z")
	router := mux.NewRouter()
	DashboardRouter(router, db)

	for _, path := range []string{
		"/nodes/node-a/metrics/cpu/last",
		"/namespaces/default/pod-list/pod-a/metrics/cpu/last",
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK {
			t.Errorf("GET %s status = %d, body=%s", path, recorder.Code, recorder.Body.String())
		}
		var response SidecarMetricResultList
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil || len(response.Items) != 1 {
			t.Errorf("GET %s response = %s, error=%v", path, recorder.Body.String(), err)
		}
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/unknown?value=<script>", nil))
	if recorder.Code != http.StatusOK || recorder.Body.Len() == 0 {
		t.Fatalf("default route status/body = %d/%q", recorder.Code, recorder.Body.String())
	}

	brokenDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer brokenDB.Close()
	badRouter := mux.NewRouter()
	DashboardRouter(badRouter, brokenDB)
	for _, path := range []string{
		"/nodes/node-a/metrics/cpu/last",
		"/namespaces/default/pod-list/pod-a/metrics/cpu/last",
	} {
		recorder := httptest.NewRecorder()
		badRouter.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusInternalServerError {
			t.Errorf("GET %s with broken DB status = %d", path, recorder.Code)
		}
	}
}

func newDashboardTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.CreateDatabase(db); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func insertMetric(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("insert metric: %v", err)
	}
}
