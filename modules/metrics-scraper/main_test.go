// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

package main

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ktesting "k8s.io/client-go/testing"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
	_ "modernc.org/sqlite"

	"k8s.io/dashboard/metrics-scraper/pkg/database"
)

func TestUpdateScrapesClusterMetricsAndWritesDatabase(t *testing.T) {
	client := metricsfake.NewSimpleClientset()
	client.PrependReactor("list", "nodes", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, &metricsv1beta1.NodeMetricsList{Items: []metricsv1beta1.NodeMetrics{{
			ObjectMeta: metav1.ObjectMeta{Name: "node-a", UID: "node-uid"},
			Usage:      corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("250m"), corev1.ResourceMemory: resource.MustParse("128Mi")},
		}}}, nil
	})
	client.PrependReactor("list", "pods", func(action ktesting.Action) (bool, runtime.Object, error) {
		if got := action.GetNamespace(); got != "" {
			t.Errorf("cluster pod metrics namespace = %q, want all namespaces", got)
		}
		return true, &metricsv1beta1.PodMetricsList{Items: []metricsv1beta1.PodMetrics{{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-a", Namespace: "default", UID: "pod-uid"},
			Containers: []metricsv1beta1.ContainerMetrics{{Name: "container-a", Usage: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("50m"), corev1.ResourceMemory: resource.MustParse("64Mi"),
			}}},
		}}}, nil
	})

	db := newUpdateTestDB(t)
	if err := update(client, db, time.Hour, []string{""}); err != nil {
		t.Fatalf("update() error = %v", err)
	}
	assertCount(t, db, "nodes", 1)
	assertCount(t, db, "pods", 1)
	assertValue(t, db, "SELECT cpu FROM nodes WHERE name='node-a'", 250)
	assertValue(t, db, "SELECT cpu FROM pods WHERE name='pod-a'", 50)
}

func TestUpdateSkipsNodesAndCombinesNamespaces(t *testing.T) {
	client := metricsfake.NewSimpleClientset()
	var listedNamespaces []string
	client.PrependReactor("list", "nodes", func(ktesting.Action) (bool, runtime.Object, error) {
		t.Fatal("node metrics should not be requested for namespace-scoped scraping")
		return true, nil, nil
	})
	client.PrependReactor("list", "pods", func(action ktesting.Action) (bool, runtime.Object, error) {
		namespace := action.GetNamespace()
		listedNamespaces = append(listedNamespaces, namespace)
		return true, &metricsv1beta1.PodMetricsList{Items: []metricsv1beta1.PodMetrics{{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-" + namespace, Namespace: namespace},
			Containers: []metricsv1beta1.ContainerMetrics{{Name: "container-a"}},
		}}}, nil
	})

	db := newUpdateTestDB(t)
	if err := update(client, db, time.Hour, []string{"team-a", "team-b"}); err != nil {
		t.Fatalf("update() error = %v", err)
	}
	if len(listedNamespaces) != 2 || listedNamespaces[0] != "team-a" || listedNamespaces[1] != "team-b" {
		t.Fatalf("listed namespaces = %v", listedNamespaces)
	}
	assertCount(t, db, "nodes", 0)
	assertCount(t, db, "pods", 2)
}

func TestUpdateReturnsScrapeAndDatabaseErrors(t *testing.T) {
	wantErr := errors.New("metrics API unavailable")
	client := metricsfake.NewSimpleClientset()
	client.PrependReactor("list", "nodes", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, wantErr
	})
	db := newUpdateTestDB(t)
	if err := update(client, db, time.Hour, []string{""}); !errors.Is(err, wantErr) {
		t.Fatalf("node scrape error = %v, want %v", err, wantErr)
	}

	client = metricsfake.NewSimpleClientset()
	client.PrependReactor("list", "pods", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, wantErr
	})
	if err := update(client, db, time.Hour, []string{"team-a"}); !errors.Is(err, wantErr) {
		t.Fatalf("pod scrape error = %v, want %v", err, wantErr)
	}

	client = metricsfake.NewSimpleClientset()
	if _, err := db.Exec("DROP TABLE nodes"); err != nil {
		t.Fatal(err)
	}
	if err := update(client, db, time.Hour, []string{"team-a"}); err == nil {
		t.Fatal("update() succeeded after its nodes table was removed")
	}
}

func newUpdateTestDB(t *testing.T) *sql.DB {
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

func assertCount(t *testing.T, db *sql.DB, table string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s row count = %d, want %d", table, got, want)
	}
}

func assertValue(t *testing.T, db *sql.DB, query string, want int64) {
	t.Helper()
	var got int64
	if err := db.QueryRow(query).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("query %q value = %d, want %d", query, got, want)
	}
}
