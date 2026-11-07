// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

package errors

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/emicklei/go-restful/v3"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestStatusErrorConstructors(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		code   int32
		reason metav1.StatusReason
	}{
		{"unauthorized", NewUnauthorized("login"), http.StatusUnauthorized, metav1.StatusReasonUnauthorized},
		{"forbidden", NewForbidden("pod", errors.New("denied")), http.StatusForbidden, metav1.StatusReasonForbidden},
		{"expired", NewTokenExpired("expired"), http.StatusUnauthorized, metav1.StatusReasonExpired},
		{"csrf", NewCSRFValidationFailed(), http.StatusUnauthorized, metav1.StatusReasonInvalid},
		{"bad request", NewBadRequest("bad"), http.StatusBadRequest, metav1.StatusReasonBadRequest},
		{"invalid", NewInvalid("invalid"), http.StatusInternalServerError, metav1.StatusReasonInvalid},
		{"not found", NewNotFound("missing"), http.StatusNotFound, metav1.StatusReasonNotFound},
		{"internal", NewInternal("broken"), http.StatusInternalServerError, metav1.StatusReasonInternalError},
		{"unexpected object", NewUnexpectedObject(&runtime.Unknown{TypeMeta: runtime.TypeMeta{Kind: "Pod"}}), http.StatusInternalServerError, metav1.StatusReasonInternalError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, ok := tc.err.(*k8serrors.StatusError)
			if !ok {
				t.Fatalf("constructor returned %T, want StatusError", tc.err)
			}
			if status.ErrStatus.Code != tc.code || status.ErrStatus.Reason != tc.reason {
				t.Errorf("status = %d/%s, want %d/%s", status.ErrStatus.Code, status.ErrStatus.Reason, tc.code, tc.reason)
			}
		})
	}
}

func TestNewGenericResponse(t *testing.T) {
	cases := []struct {
		code    int
		reason  metav1.StatusReason
		message string
	}{
		{http.StatusNotFound, metav1.StatusReasonNotFound, "the server could not find the requested resource"},
		{http.StatusBadRequest, metav1.StatusReasonBadRequest, "the server rejected our request for an unknown reason"},
		{http.StatusUnauthorized, metav1.StatusReasonUnauthorized, "the server has asked for the client to provide credentials"},
		{http.StatusForbidden, metav1.StatusReasonForbidden, "details"},
		{http.StatusConflict, metav1.StatusReasonConflict, "details"},
		{http.StatusNotAcceptable, metav1.StatusReasonNotAcceptable, "details"},
		{http.StatusUnsupportedMediaType, metav1.StatusReasonUnsupportedMediaType, "details"},
		{http.StatusMethodNotAllowed, metav1.StatusReasonMethodNotAllowed, "the server does not allow this method on the requested resource"},
		{http.StatusUnprocessableEntity, metav1.StatusReasonInvalid, "the server rejected our request due to an error in our request"},
		{http.StatusServiceUnavailable, metav1.StatusReasonServiceUnavailable, "the server is currently unable to handle the request"},
		{http.StatusGatewayTimeout, metav1.StatusReasonTimeout, "the server was unable to return a response in the time allotted, but may still be processing the request"},
		{http.StatusTooManyRequests, metav1.StatusReasonTooManyRequests, "the server has received too many requests and has asked us to try again later"},
		{http.StatusInternalServerError, metav1.StatusReasonInternalError, `an error on the server ("details") has prevented the request from succeeding`},
		{http.StatusTeapot, metav1.StatusReasonUnknown, "the server responded with the status code 418 but did not return more information"},
	}
	for _, tc := range cases {
		got := NewGenericResponse(tc.code, "details")
		if got.ErrStatus.Code != int32(tc.code) || got.ErrStatus.Reason != tc.reason || got.ErrStatus.Message != tc.message {
			t.Errorf("NewGenericResponse(%d) = %#v", tc.code, got.ErrStatus)
		}
	}
}

func TestErrorClassificationAndLocalization(t *testing.T) {
	if !IsTokenExpired(NewTokenExpired(MsgTokenExpiredError)) || IsTokenExpired(NewBadRequest("other")) {
		t.Fatal("token expiration classification was incorrect")
	}
	if !IsAlreadyExists(k8serrors.NewAlreadyExists(schema.GroupResource{Resource: "pods"}, "web")) || IsAlreadyExists(errors.New("other")) {
		t.Fatal("already-exists classification was incorrect")
	}
	if !IsUnauthorized(NewUnauthorized("auth")) || IsUnauthorized(errors.New("other")) {
		t.Fatal("unauthorized classification was incorrect")
	}
	if !IsForbidden(NewForbidden("pod", errors.New("denied"))) || IsForbidden(errors.New("other")) {
		t.Fatal("forbidden classification was incorrect")
	}
	if !IsNotFound(NewNotFound("pod")) || IsNotFound(errors.New("other")) {
		t.Fatal("not-found classification was incorrect")
	}

	localized := []struct{ input, want string }{
		{"resource does not match the namespace of target", MsgDeployNamespaceMismatchError},
		{"empty namespace may not be set", MsgDeployEmptyNamespaceError},
	}
	for _, tc := range localized {
		got := LocalizeError(NewBadRequest(tc.input))
		var status *k8serrors.StatusError
		if !k8serrors.IsBadRequest(got) || !errors.As(got, &status) || status.ErrStatus.Message != tc.want {
			t.Errorf("LocalizeError(%q) = %v", tc.input, got)
		}
	}
	if got := LocalizeError(NewGenericResponse(http.StatusUnauthorized, "")); !IsUnauthorized(got) || got.Error() != MsgLoginUnauthorizedError {
		t.Errorf("localized unauthorized error = %v", got)
	}
	if LocalizeError(nil) != nil || LocalizeError(errors.New("unmapped")).Error() != "unmapped" {
		t.Fatal("unmapped or nil errors were changed")
	}
}

func TestErrorAggregationAndHandling(t *testing.T) {
	nonCritical := NewForbidden("pod", errors.New("denied"))
	items, critical := ExtractErrors(nonCritical)
	if critical != nil || len(items) != 1 || items[0].Error() != nonCritical.Error() {
		t.Fatalf("ExtractErrors(forbidden) = %v, %v", items, critical)
	}
	warnings := items
	if items, critical = ExtractErrors(nil); critical != nil || len(items) != 0 {
		t.Fatalf("ExtractErrors(nil) = %v, %v", items, critical)
	}
	if items, critical = ExtractErrors(errors.New("unexpected")); critical == nil || len(items) != 0 {
		t.Fatalf("ExtractErrors(critical) = %v, %v", items, critical)
	}

	merged := MergeErrors(warnings, []error{nonCritical, fmt.Errorf("another")}, []error{fmt.Errorf("another")})
	if len(merged) != 2 || merged[0].Error() != nonCritical.Error() || merged[1].Error() != "another" {
		t.Fatalf("MergeErrors() = %v", merged)
	}

	if code, err := HandleError(NewUnauthorized("bad")); code != http.StatusUnauthorized || err.Error() != MsgLoginUnauthorizedError {
		t.Errorf("HandleError(unauthorized) = %d, %v", code, err)
	}
	if code, err := HandleError(nonCritical); code != http.StatusForbidden || !k8serrors.IsForbidden(err) {
		t.Errorf("HandleError(forbidden) = %d, %v", code, err)
	}
	ordinary := errors.New("ordinary")
	if code, err := HandleError(ordinary); code != http.StatusInternalServerError || err != ordinary {
		t.Errorf("HandleError(ordinary) = %d, %v", code, err)
	}

	recorder := httptest.NewRecorder()
	response := restful.NewResponse(recorder)
	HandleInternalError(response, ordinary)
	if response.StatusCode() != http.StatusInternalServerError || recorder.Code != http.StatusInternalServerError {
		t.Errorf("HandleInternalError status = %d/%d", response.StatusCode(), recorder.Code)
	}
}
