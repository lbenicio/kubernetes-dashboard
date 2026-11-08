// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package handler

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/emicklei/go-restful/v3"
	"k8s.io/dashboard/client"
)

func TestHandlersReturnErrorsWhenNoRequestCredentialsAreAvailable(t *testing.T) {
	tests := []struct {
		name string
		call func(*APIHandler, *restful.Request, *restful.Response)
	}{
		{name: "node drain", call: (*APIHandler).handleNodeDrain},
		{name: "deploy application", call: (*APIHandler).handleDeploy},
		{name: "scale resource", call: (*APIHandler).handleScaleResource},
		{name: "deploy from file", call: (*APIHandler).handleDeployFromFile},
		{name: "pause deployment", call: (*APIHandler).handleDeploymentPause},
		{name: "rollback deployment", call: (*APIHandler).handleDeploymentRollback},
		{name: "restart deployment", call: (*APIHandler).handleDeploymentRestart},
		{name: "resume deployment", call: (*APIHandler).handleDeploymentResume},
		{name: "validate app name", call: (*APIHandler).handleNameValidity},
		{name: "update replicas", call: (*APIHandler).handleUpdateReplicasCount},
		{name: "get raw resource", call: (*APIHandler).handleGetResource},
		{name: "update raw resource", call: (*APIHandler).handlePutResource},
		{name: "delete raw resource", call: (*APIHandler).handleDeleteResource},
		{name: "create namespace", call: (*APIHandler).handleCreateNamespace},
		{name: "create image pull secret", call: (*APIHandler).handleCreateImagePullSecret},
		{name: "restart daemonset", call: (*APIHandler).handleDaemonSetRestart},
		{name: "restart statefulset", call: (*APIHandler).handleStatefulSetRestart},
		{name: "trigger cronjob", call: (*APIHandler).handleTriggerCronJob},
		{name: "custom resource definition detail", call: (*APIHandler).handleGetCustomResourceDefinitionDetail},
		{name: "custom resource object list", call: (*APIHandler).handleGetCustomResourceObjectList},
		{name: "custom resource object detail", call: (*APIHandler).handleGetCustomResourceObjectDetail},
		{name: "log source", call: (*APIHandler).handleLogSource},
		{name: "logs", call: (*APIHandler).handleLogs},
		{name: "log file", call: (*APIHandler).handleLogFile},
	}

	apiHandler := &APIHandler{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := restful.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
			recorder := httptest.NewRecorder()
			test.call(apiHandler, request, restful.NewResponse(recorder))
			if recorder.Code < http.StatusBadRequest || recorder.Body.Len() == 0 {
				t.Fatalf("handler response = %d %q; want an error response", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestPutResourceReturnsRequestBodyReadFailure(t *testing.T) {
	client.Init(client.WithMasterUrl("https://kubernetes.example.test"))
	request := restful.NewRequest(httptest.NewRequest(http.MethodPut, "/api/v1/_raw/pod/namespace/default/name/web", nil))
	request.Request.Header.Set("Authorization", "Bearer raw-body-read-error-test-token")
	request.Request.Body = io.NopCloser(failingRequestBody{})
	recorder := httptest.NewRecorder()
	(&APIHandler{}).handlePutResource(request, restful.NewResponse(recorder))
	if recorder.Code != http.StatusInternalServerError || recorder.Body.Len() == 0 {
		t.Fatalf("handlePutResource() response = %d %q; want a request body error", recorder.Code, recorder.Body.String())
	}
}

type failingRequestBody struct{}

func (failingRequestBody) Read([]byte) (int, error) { return 0, errors.New("request body read failed") }
