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

package main

import (
	"database/sql"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/dashboard/metrics-scraper/pkg/database"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

func TestInitializeScraperBuildsMetricsClientAndDatabase(t *testing.T) {
	kubeconfig := writeScraperKubeconfig(t, "https://kubernetes.example.test")
	clientset, db, err := initializeScraper(kubeconfig, filepath.Join(t.TempDir(), "metrics.db"))
	if err != nil {
		t.Fatalf("initializeScraper() error = %v", err)
	}
	if clientset == nil || db == nil {
		t.Fatalf("initializeScraper() returned client %v and database %v", clientset, db)
	}
	defer db.Close()
	var table string
	if err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='pods'").Scan(&table); err != nil || table != "pods" {
		t.Fatalf("initialized database pods table = %q, %v", table, err)
	}
}

func TestRunScraperMainHandlesInitializationAndStartsService(t *testing.T) {
	if err := runScraperMain(filepath.Join(t.TempDir(), "missing-kubeconfig"), filepath.Join(t.TempDir(), "unused.db"), time.Hour, time.Minute, []string{"default"}, closedScraperQuit(), func(string, http.Handler) error {
		t.Fatal("listener started after scraper initialization failed")
		return nil
	}); err == nil || !strings.Contains(err.Error(), "client config") {
		t.Fatalf("runScraperMain(missing config) error = %v", err)
	}

	apiHandlers := make(chan http.Handler, 1)
	listenAndServe := func(address string, handler http.Handler) error {
		if address != ":8000" {
			t.Errorf("metrics API address = %q; want :8000", address)
		}
		apiHandlers <- handler
		return nil
	}
	if err := runScraperMain(writeScraperKubeconfig(t, "https://kubernetes.example.test"), filepath.Join(t.TempDir(), "metrics.db"), time.Hour, time.Minute, []string{"default"}, closedScraperQuit(), listenAndServe); err != nil {
		t.Fatalf("runScraperMain(valid config) error = %v", err)
	}
	select {
	case handler := <-apiHandlers:
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/unknown", nil))
		if response.Code != http.StatusOK {
			t.Errorf("metrics API fallback route status = %d", response.Code)
		}
	case <-time.After(time.Second):
		t.Fatal("metrics API listener was not started")
	}
}

func TestInitializeScraperReportsConfigClientAndDatabaseErrors(t *testing.T) {
	if clientset, db, err := initializeScraper(filepath.Join(t.TempDir(), "missing-kubeconfig"), filepath.Join(t.TempDir(), "metrics.db")); err == nil || clientset != nil || db != nil {
		t.Fatalf("initializeScraper(missing kubeconfig) = %v, %v, %v", clientset, db, err)
	}

	invalidKubeconfig := writeScraperKubeconfig(t, "%")
	if clientset, db, err := initializeScraper(invalidKubeconfig, filepath.Join(t.TempDir(), "metrics.db")); err == nil || clientset != nil || db != nil {
		t.Fatalf("initializeScraper(invalid API URL) = %v, %v, %v", clientset, db, err)
	}

	kubeconfig := writeScraperKubeconfig(t, "https://kubernetes.example.test")
	invalidDatabasePath := filepath.Join(t.TempDir(), "missing-directory", "metrics.db")
	if clientset, db, err := initializeScraper(kubeconfig, invalidDatabasePath); err == nil || clientset != nil || db != nil || !strings.Contains(err.Error(), "database tables") {
		t.Fatalf("initializeScraper(invalid database path) = %v, %v, %v", clientset, db, err)
	}

	invalidCAKubeconfig := filepath.Join(t.TempDir(), "invalid-ca-kubeconfig")
	invalidCAConfig := "apiVersion: v1\nkind: Config\nclusters:\n- name: test\n  cluster:\n    server: https://kubernetes.example.test\n    certificate-authority-data: bm90LXBlbQ==\nusers:\n- name: test\n  user:\n    token: unit-test-token\ncontexts:\n- name: test\n  context:\n    cluster: test\n    user: test\ncurrent-context: test\n"
	if err := os.WriteFile(invalidCAKubeconfig, []byte(invalidCAConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	if clientset, db, err := initializeScraper(invalidCAKubeconfig, filepath.Join(t.TempDir(), "unused.db")); err == nil || clientset != nil || db != nil || !strings.Contains(err.Error(), "clientset") {
		t.Fatalf("initializeScraper(invalid CA) = %v, %v, %v; want a metrics client construction error", clientset, db, err)
	}

}

func TestUpdateReportsClosedDatabaseErrors(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "closed.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := update(metricsfake.NewSimpleClientset(), db, time.Minute, []string{"default"}); err == nil {
		t.Fatal("update() succeeded with a closed database")
	}
}

func TestUpdateHandlesScrapeDatabaseAndCullOutcomes(t *testing.T) {
	t.Run("node metrics request fails", func(t *testing.T) {
		clientset := metricsfake.NewSimpleClientset()
		clientset.PrependReactor("list", "nodes", func(ktesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("node metrics unavailable")
		})
		if err := update(clientset, nil, time.Minute, []string{""}); err == nil || err.Error() != "node metrics unavailable" {
			t.Fatalf("update(node failure) error = %v", err)
		}
	})

	t.Run("pod metrics request fails", func(t *testing.T) {
		clientset := metricsfake.NewSimpleClientset()
		clientset.PrependReactor("list", "pods", func(ktesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("pod metrics unavailable")
		})
		if err := update(clientset, nil, time.Minute, []string{"default"}); err == nil || err.Error() != "pod metrics unavailable" {
			t.Fatalf("update(pod failure) error = %v", err)
		}
	})

	t.Run("database update and cull succeed", func(t *testing.T) {
		db := newScraperTestDatabase(t)
		if err := update(metricsfake.NewSimpleClientset(), db, time.Minute, []string{""}); err != nil {
			t.Fatalf("update(empty scrape) error = %v", err)
		}
	})

	t.Run("database cull fails", func(t *testing.T) {
		db := newScraperTestDatabase(t)
		if _, err := db.Exec("INSERT INTO nodes(name, time) VALUES ('old-node', datetime('now', '-1 hour'))"); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec("CREATE TRIGGER reject_cull BEFORE DELETE ON nodes BEGIN SELECT RAISE(ABORT, 'cull rejected'); END;"); err != nil {
			t.Fatal(err)
		}
		if err := update(metricsfake.NewSimpleClientset(), db, time.Minute, []string{"default"}); err == nil || !strings.Contains(err.Error(), "cull rejected") {
			t.Fatalf("update(cull failure) error = %v", err)
		}
	})
}

func TestRunScraperContinuesAfterFailuresAndStopsOnQuit(t *testing.T) {
	quit := make(chan struct{})
	ticks := make(chan time.Time, 2)
	updates := make(chan int, 2)
	done := make(chan struct{})
	calls := 0
	go func() {
		runScraper(quit, ticks, func() error {
			calls++
			updates <- calls
			if calls == 1 {
				return errors.New("temporary scrape error")
			}
			return nil
		})
		close(done)
	}()

	ticks <- time.Now()
	if got := <-updates; got != 1 {
		t.Fatalf("first scrape count = %d; want 1", got)
	}
	ticks <- time.Now()
	if got := <-updates; got != 2 {
		t.Fatalf("second scrape count = %d; want 2 after the first error", got)
	}
	close(quit)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scraper did not stop after quit was closed")
	}

	stopped := make(chan struct{})
	closedQuit := make(chan struct{})
	close(closedQuit)
	runScraper(closedQuit, nil, func() error {
		close(stopped)
		return nil
	})
	select {
	case <-stopped:
		t.Fatal("scraper invoked its callback after quit was closed")
	default:
	}
}

func TestRunScraperServiceWiresRoutesAndScrapesOnTick(t *testing.T) {
	db := newScraperTestDatabase(t)
	requests := make(chan string, 1)
	clientset, err := metricsclient.NewForConfig(&rest.Config{
		Host: "https://kubernetes.example.test",
		WrapTransport: func(http.RoundTripper) http.RoundTripper {
			return scraperRoundTripper(func(request *http.Request) (*http.Response, error) {
				requests <- request.URL.Path
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"kind":"PodMetricsList","apiVersion":"metrics.k8s.io/v1beta1","items":[]}`)),
					Request:    request,
				}, nil
			})
		},
	})
	if err != nil {
		t.Fatalf("create metrics client: %v", err)
	}

	apiHandlers := make(chan http.Handler, 1)
	listenAndServe := func(address string, handler http.Handler) error {
		if address != ":8000" {
			t.Errorf("metrics API address = %q; want :8000", address)
		}
		apiHandlers <- handler
		return nil
	}
	quit := make(chan struct{})
	ticks := make(chan time.Time, 1)
	done := make(chan struct{})
	go func() {
		runScraperService(clientset, db, time.Minute, []string{"default"}, ticks, quit, listenAndServe)
		close(done)
	}()
	ticks <- time.Now()

	select {
	case path := <-requests:
		if path != "/apis/metrics.k8s.io/v1beta1/namespaces/default/pods" {
			t.Errorf("scrape request path = %q", path)
		}
	case <-time.After(time.Second):
		t.Fatal("scraper did not request pod metrics after the tick")
	}
	close(quit)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scraper service did not stop after quit")
	}

	select {
	case handler := <-apiHandlers:
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/unknown", nil))
		if response.Code != http.StatusOK {
			t.Errorf("metrics API fallback route status = %d", response.Code)
		}
	case <-time.After(time.Second):
		t.Fatal("metrics API listener was not started")
	}
}

type scraperRoundTripper func(*http.Request) (*http.Response, error)

func (in scraperRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return in(request)
}

func newScraperTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "metrics.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if err := database.CreateDatabase(db); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func writeScraperKubeconfig(t *testing.T, server string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "kubeconfig")
	config := "apiVersion: v1\nkind: Config\nclusters:\n- name: test\n  cluster:\n    server: " + server + "\nusers:\n- name: test\n  user:\n    token: unit-test-token\ncontexts:\n- name: test\n  context:\n    cluster: test\n    user: test\ncurrent-context: test\n"
	if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	return path
}

func closedScraperQuit() <-chan struct{} {
	quit := make(chan struct{})
	close(quit)
	return quit
}
