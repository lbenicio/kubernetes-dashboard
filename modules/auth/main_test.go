// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

func TestInitializeAuthWithoutOIDC(t *testing.T) {
	for name, value := range map[string]string{
		"apiserver-host":        "https://kubernetes.example.test",
		"kubeconfig":            "",
		"oidc-issuer-url":       "",
		"oidc-client-id":        "",
		"oidc-client-secret":    "",
		"oidc-redirect-url":     "",
		"anonymous-admin":       "false",
		"session-cookie-secure": "false",
	} {
		setAuthMainFlag(t, name, value)
	}
	if err := initializeAuth(); err != nil {
		t.Fatalf("initializeAuth() with OIDC disabled = %v", err)
	}
}

func TestInitializeAuthReturnsOIDCConfigurationError(t *testing.T) {
	setAuthMainFlag(t, "apiserver-host", "https://kubernetes.example.test")
	setAuthMainFlag(t, "kubeconfig", "")
	setAuthMainFlag(t, "oidc-issuer-url", "https://identity.example.test")
	setAuthMainFlag(t, "oidc-client-id", "")
	setAuthMainFlag(t, "oidc-client-secret", "")
	setAuthMainFlag(t, "oidc-redirect-url", "")
	setAuthMainFlag(t, "anonymous-admin", "false")
	if err := initializeAuth(); err == nil || !strings.Contains(err.Error(), "requires issuer URL") {
		t.Fatalf("initializeAuth() with partial OIDC config = %v", err)
	}
}

func TestRunAuthInitializationAndServingOutcomes(t *testing.T) {
	initializationErr := errors.New("OIDC initialization failed")
	serveErr := errors.New("listener failed")
	for _, test := range []struct {
		name            string
		initializeErr   error
		serveErr        error
		wantInitialized bool
		wantErr         error
	}{
		{name: "serves after initialization", wantInitialized: true},
		{name: "returns initialization error", initializeErr: initializationErr, wantErr: initializationErr},
		{name: "returns router error", serveErr: serveErr, wantInitialized: true, wantErr: serveErr},
	} {
		t.Run(test.name, func(t *testing.T) {
			called := false
			initialized, err := runAuth(func() error { return test.initializeErr }, func(address string) error {
				called = true
				if address != ":9443" {
					t.Errorf("serve address = %q, want :9443", address)
				}
				return test.serveErr
			}, ":9443")
			if initialized != test.wantInitialized || !errors.Is(err, test.wantErr) {
				t.Fatalf("runAuth() = %t, %v; want %t, %v", initialized, err, test.wantInitialized, test.wantErr)
			}
			if called == (test.initializeErr != nil) {
				t.Errorf("serve called = %t with initialization error %v", called, test.initializeErr)
			}
		})
	}
}

func TestRunAuthProcessExitsOnlyAfterStartupFailure(t *testing.T) {
	for _, test := range []struct {
		name          string
		initializeErr error
		serveErr      error
		wantExit      bool
	}{
		{name: "successful server return"},
		{name: "initialization failure", initializeErr: errors.New("OIDC config error"), wantExit: true},
		{name: "router failure", serveErr: errors.New("router error"), wantExit: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			calledServe := false
			exitCode := 0
			exited := false
			runAuthProcess(func() error { return test.initializeErr }, func(string) error {
				calledServe = true
				return test.serveErr
			}, ":9443", func(code int) { exited = true; exitCode = code })
			if exited != test.wantExit || (exited && exitCode != 1) {
				t.Fatalf("exit callback = %t/%d; want %t/1", exited, exitCode, test.wantExit)
			}
			if calledServe == (test.initializeErr != nil) {
				t.Errorf("serve called = %t with initialization error %v", calledServe, test.initializeErr)
			}
		})
	}
}

func setAuthMainFlag(t *testing.T, name, value string) {
	t.Helper()
	flag := pflag.Lookup(name)
	if flag == nil {
		t.Fatalf("flag %q is not registered", name)
	}
	previous := flag.Value.String()
	t.Cleanup(func() { _ = flag.Value.Set(previous) })
	if err := flag.Value.Set(value); err != nil {
		t.Fatalf("set flag %s: %v", name, err)
	}
}
