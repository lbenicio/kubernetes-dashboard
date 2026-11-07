// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

package settings

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	v1 "k8s.io/api/authorization/v1"
)

func TestToSelfSubjectAccessReview(t *testing.T) {
	review := ToSelfSubjectAccessReview("dashboard", "settings", "ConfigMap", "PATCH")
	want := &v1.SelfSubjectAccessReview{
		Spec: v1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &v1.ResourceAttributes{
				Namespace: "dashboard",
				Name:      "settings",
				Resource:  "configmaps",
				Verb:      "patch",
			},
		},
	}
	if review.Spec.ResourceAttributes == nil || *review.Spec.ResourceAttributes != *want.Spec.ResourceAttributes {
		t.Fatalf("review attributes = %#v, want %#v", review.Spec.ResourceAttributes, want.Spec.ResourceAttributes)
	}
}

func TestSettingsMutationHandlersReportUninitializedClient(t *testing.T) {
	cases := []struct {
		name    string
		handler gin.HandlerFunc
		method  string
		path    string
		body    string
	}{
		{name: "save settings", handler: handleSettingsGlobalSave, method: http.MethodPut, path: "/settings", body: `{}`},
		{name: "save pinned resource", handler: handleSettingsSavePinned, method: http.MethodPut, path: "/settings/pinnedresources", body: `{}`},
		{name: "delete pinned resource", handler: handleSettingsDeletePinned, method: http.MethodDelete, path: "/settings/pinnedresources/pod/default/web"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			tc.handler(context)
			if recorder.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
			}
		})
	}
}

func TestSettingsCanIHandlerReturnsDeniedWithoutInitializedClient(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/settings/cani", nil)

	handleGetSettingsGlobalCanI(context)

	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"allowed":false`) {
		t.Fatalf("CanI response = %d %s", recorder.Code, recorder.Body.String())
	}
}
