// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package args

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestInitializeOptionalHandlersStartsConfiguredEndpoints(t *testing.T) {
	type startedServer struct {
		address string
		handler http.Handler
	}
	started := make(chan startedServer, 2)
	listenAndServe := func(address string, handler http.Handler) error {
		started <- startedServer{address: address, handler: handler}
		return nil
	}

	initializeOptionalHandlers(false, false, listenAndServe)
	select {
	case server := <-started:
		t.Fatalf("disabled handlers unexpectedly started server on %s", server.address)
	default:
	}

	initializeOptionalHandlers(true, true, listenAndServe)
	servers := make(map[string]http.Handler, 2)
	for range 2 {
		select {
		case server := <-started:
			servers[server.address] = server.handler
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for optional handler startup")
		}
	}

	for _, test := range []struct {
		address string
		path    string
	}{
		{address: ":8070", path: "/debug/pprof/"},
		{address: ":8080", path: "/metrics"},
	} {
		handler := servers[test.address]
		if handler == nil {
			t.Fatalf("no handler started on %s", test.address)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
		if response.Code != http.StatusOK {
			t.Errorf("%s handler status = %d; want %d", test.path, response.Code, http.StatusOK)
		}
	}
}
