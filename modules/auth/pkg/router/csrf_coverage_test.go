// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"k8s.io/dashboard/csrf"
)

func TestRouterCSRFAllowsOIDCCallbackAndRejectsOtherPosts(t *testing.T) {
	csrf.Ensure()
	V1().POST("/csrf-coverage", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	V1().POST("/oidc/callback", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	for _, test := range []struct {
		path string
		want int
	}{
		{path: "/api/v1/csrf-coverage", want: http.StatusUnauthorized},
		{path: "/api/v1/oidc/callback", want: http.StatusNoContent},
	} {
		t.Run(test.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, test.path, nil)
			Router().ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("POST %s status = %d; want %d", test.path, response.Code, test.want)
			}
		})
	}
}
