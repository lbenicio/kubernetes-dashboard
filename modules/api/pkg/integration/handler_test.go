// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

package integration

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/emicklei/go-restful/v3"

	"k8s.io/dashboard/api/pkg/integration/api"
	"k8s.io/dashboard/api/pkg/integration/metric"
)

type testIntegrationManager struct {
	state *api.IntegrationState
	err   error
	id    api.IntegrationID
}

func (m *testIntegrationManager) GetState(id api.IntegrationID) (*api.IntegrationState, error) {
	m.id = id
	return m.state, m.err
}

func (*testIntegrationManager) List() []api.Integration { return nil }

func (*testIntegrationManager) Metric() metric.MetricManager { return nil }

func serveIntegrationState(t *testing.T, manager Manager) *httptest.ResponseRecorder {
	t.Helper()
	ws := new(restful.WebService)
	ws.Path("/api/v1").Produces(restful.MIME_JSON)
	NewHandler(manager).Install(ws)
	container := restful.NewContainer()
	container.Add(ws)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/integration/sidecar/state", nil)
	request.Header.Set("Accept", restful.MIME_JSON)
	container.ServeHTTP(response, request)
	return response
}

func TestIntegrationStateHandlerReturnsState(t *testing.T) {
	manager := &testIntegrationManager{state: &api.IntegrationState{Connected: true}}
	response := serveIntegrationState(t, manager)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if response.Body.Len() == 0 {
		t.Error("successful response body is empty")
	}
	if manager.id != api.SidecarIntegrationID {
		t.Errorf("manager received integration ID %q, want %q", manager.id, api.SidecarIntegrationID)
	}
}

func TestIntegrationStateHandlerReturnsManagerError(t *testing.T) {
	response := serveIntegrationState(t, &testIntegrationManager{err: errors.New("unknown integration")})
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if response.Body.String() != "unknown integration\n" {
		t.Errorf("body = %q, want manager error", response.Body.String())
	}
}
