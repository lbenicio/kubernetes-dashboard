// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package sidecar

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	kubescheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestSidecarRESTClientRequests(t *testing.T) {
	var paths []string
	restClient, err := rest.RESTClientFor(&rest.Config{
		Host:    "http://metrics.test",
		APIPath: "/api",
		ContentConfig: rest.ContentConfig{
			GroupVersion:         &schema.GroupVersion{Version: "v1"},
			NegotiatedSerializer: kubescheme.Codecs.WithoutConversion(),
		},
		UserAgent: "sidecar-test",
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			paths = append(paths, request.URL.Path)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("ok")),
				Request:    request,
			}, nil
		}),
	})
	if err != nil {
		t.Fatalf("create REST client: %v", err)
	}

	remote := remoteSidecarClient{client: restClient}
	if _, err := remote.Get("metrics/pods").DoRaw(context.Background()); err != nil {
		t.Fatalf("remote Get().DoRaw() error = %v", err)
	}
	if err := remote.HealthCheck(); err != nil {
		t.Fatalf("remote HealthCheck() error = %v", err)
	}
	inCluster := inClusterSidecarClient{client: restClient}
	if _, err := inCluster.Get("metrics/pods").DoRaw(context.Background()); err != nil {
		t.Fatalf("in-cluster Get().DoRaw() error = %v", err)
	}
	if err := inCluster.HealthCheck(); err != nil {
		t.Fatalf("in-cluster HealthCheck() error = %v", err)
	}
	if len(paths) != 4 {
		t.Fatalf("transport received %d requests, want 4: %v", len(paths), paths)
	}
	if paths[0] != "/api/v1/metrics/pods" || paths[1] != "/healthz" ||
		!strings.Contains(paths[2], "/services/") || !strings.HasSuffix(paths[2], "/metrics/pods") ||
		!strings.Contains(paths[3], "/services/") || !strings.HasSuffix(paths[3], "/healthz") {
		t.Errorf("REST client request paths = %v", paths)
	}
}
