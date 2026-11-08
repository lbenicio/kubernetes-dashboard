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

package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"time"

	"k8s.io/klog/v2"

	_ "modernc.org/sqlite"

	"k8s.io/dashboard/metrics-scraper/pkg/api"
	"k8s.io/dashboard/metrics-scraper/pkg/args"
	"k8s.io/dashboard/metrics-scraper/pkg/database"
	"k8s.io/dashboard/metrics-scraper/pkg/environment"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

func main() {
	klog.InfoS("Starting Metrics Scraper", "version", environment.Version)

	if err := runScraperMain(args.KubeconfigPath(), args.DBFile(), args.MetricResolution(), args.MetricDuration(), args.MetricNamespaces(), make(chan struct{}), http.ListenAndServe); err != nil {
		klog.Fatalf("Unable to initialize Metrics Scraper: %s", err)
	}
}

func runScraperMain(
	kubeconfigPath, databasePath string,
	metricResolution, metricDuration time.Duration,
	metricNamespaces []string,
	quit <-chan struct{},
	listenAndServe func(string, http.Handler) error,
) error {
	clientset, db, err := initializeScraper(kubeconfigPath, databasePath)
	if err != nil {
		return err
	}
	defer db.Close()

	ticker := time.NewTicker(metricResolution)
	defer ticker.Stop()
	runScraperService(clientset, db, metricDuration, metricNamespaces, ticker.C, quit, listenAndServe)
	return nil
}

func runScraperService(
	clientset metricsclient.Interface,
	db *sql.DB,
	metricDuration time.Duration,
	metricNamespaces []string,
	ticks <-chan time.Time,
	quit <-chan struct{},
	listenAndServe func(string, http.Handler) error,
) {
	klog.Infof("Kubernetes host: %s", clientset.Discovery().RESTClient().Get().URL().Host)
	klog.Infof("Namespace(s): %s", metricNamespaces)

	go func() {
		r := mux.NewRouter()

		api.Manager(r, db)
		// Bind to a port and pass our router in
		if err := listenAndServe(":8000", handlers.CombinedLoggingHandler(os.Stdout, r)); err != nil {
			klog.Fatal(err)
		}
	}()

	runScraper(quit, ticks, func() error {
		return update(clientset, db, metricDuration, metricNamespaces)
	})
}

func runScraper(quit <-chan struct{}, ticks <-chan time.Time, scrape func() error) {
	for {
		select {
		case <-quit:
			return

		case <-ticks:
			if err := scrape(); err != nil {
				break
			}
		}
	}
}

func initializeScraper(kubeconfigPath, databasePath string) (metricsclient.Interface, *sql.DB, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to generate a client config: %w", err)
	}
	clientset, err := metricsclient.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to generate a clientset: %w", err)
	}
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to open SQLite database: %w", err)
	}
	if err := database.CreateDatabase(db); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("unable to initialize database tables: %w", err)
	}
	return clientset, db, nil
}

/**
* Update the Node and Pod metrics in the provided DB
 */
func update(client metricsclient.Interface, db *sql.DB, metricDuration time.Duration, metricNamespaces []string) error {
	nodeMetrics := &v1beta1.NodeMetricsList{}
	podMetrics := &v1beta1.PodMetricsList{}
	ctx := context.TODO()
	var err error

	// If no namespace is provided, make a call to the Node
	if len(metricNamespaces) == 1 && (metricNamespaces)[0] == "" {
		// List node metrics across the cluster
		nodeMetrics, err = client.MetricsV1beta1().NodeMetricses().List(ctx, v1.ListOptions{})
		if err != nil {
			klog.Errorf("Error scraping node metrics: %s", err)
			return err
		}
	}

	// List pod metrics across the cluster, or for a given namespace
	for _, namespace := range metricNamespaces {
		pod, err := client.MetricsV1beta1().PodMetricses(namespace).List(ctx, v1.ListOptions{})
		if err != nil {
			klog.Errorf("Error scraping '%s' for pod metrics: %s", namespace, err)
			return err
		}
		podMetrics.TypeMeta = pod.TypeMeta
		podMetrics.ListMeta = pod.ListMeta
		podMetrics.Items = append(podMetrics.Items, pod.Items...)
	}

	// Insert scrapes into DB
	err = database.UpdateDatabase(db, nodeMetrics, podMetrics)
	if err != nil {
		klog.Errorf("Error updating database: %s", err)
		return err
	}

	// Delete rows outside of the metricDuration time
	err = database.CullDatabase(db, metricDuration)
	if err != nil {
		klog.Errorf("Error culling database: %s", err)
		return err
	}

	klog.Infof("Database updated: %d nodes, %d pods", len(nodeMetrics.Items), len(podMetrics.Items))
	return nil
}
