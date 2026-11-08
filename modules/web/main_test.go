// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
)

func TestRunWebSelectsServerModeAndPropagatesFailures(t *testing.T) {
	certificateErr := errors.New("certificate paths unavailable")
	serverErr := errors.New("listener failed")
	for _, test := range []struct {
		name            string
		certPath        string
		keyPath         string
		certificateErr  error
		serveErr        error
		wantInitialized bool
		wantMode        string
		wantErr         error
	}{
		{name: "TLS", certPath: "/tmp/server.crt", keyPath: "/tmp/server.key", wantInitialized: true, wantMode: "tls"},
		{name: "insecure when certificates are absent", wantInitialized: true, wantMode: "insecure"},
		{name: "insecure when only one certificate path exists", certPath: "/tmp/server.crt", wantInitialized: true, wantMode: "insecure"},
		{name: "certificate error", certificateErr: certificateErr, wantErr: certificateErr},
		{name: "TLS listener error", certPath: "/tmp/server.crt", keyPath: "/tmp/server.key", serveErr: serverErr, wantInitialized: true, wantMode: "tls", wantErr: serverErr},
		{name: "HTTP listener error", serveErr: serverErr, wantInitialized: true, wantMode: "insecure", wantErr: serverErr},
	} {
		t.Run(test.name, func(t *testing.T) {
			initialized := 0
			mode := ""
			gotAddress, gotCert, gotKey := "", "", ""
			wasInitialized, err := runWeb(
				func() { initialized++ },
				func() (string, string, error) { return test.certPath, test.keyPath, test.certificateErr },
				func(address, cert, key string) error {
					mode, gotAddress, gotCert, gotKey = "tls", address, cert, key
					return test.serveErr
				},
				func(address string) error {
					mode, gotAddress = "insecure", address
					return test.serveErr
				},
				":8443", ":8000",
			)
			if initialized != 1 || wasInitialized != test.wantInitialized || !errors.Is(err, test.wantErr) {
				t.Fatalf("runWeb() = initialized %d/%t, error %v; want 1/%t, %v", initialized, wasInitialized, err, test.wantInitialized, test.wantErr)
			}
			if mode != test.wantMode {
				t.Fatalf("selected server mode = %q, want %q", mode, test.wantMode)
			}
			if test.wantMode == "tls" && (gotAddress != ":8443" || gotCert != test.certPath || gotKey != test.keyPath) {
				t.Errorf("TLS args = %q, %q, %q", gotAddress, gotCert, gotKey)
			}
			if test.wantMode == "insecure" && gotAddress != ":8000" {
				t.Errorf("HTTP address = %q, want :8000", gotAddress)
			}
		})
	}
}

func TestRunWebProcessReportsFailuresThroughCallback(t *testing.T) {
	certificateErr := errors.New("certificate files unavailable")
	serverErr := errors.New("router stopped")
	for _, test := range []struct {
		name           string
		certificateErr error
		serveErr       error
		wantFatal      bool
		wantSecure     bool
	}{
		{name: "success"},
		{name: "certificate failure", certificateErr: certificateErr, wantFatal: true},
		{name: "listener failure", serveErr: serverErr, wantFatal: true, wantSecure: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			fatalCalled := false
			var reported error
			var initialized bool
			runWebProcess(func() {}, func() (string, string, error) {
				return "/tmp/tls.crt", "/tmp/tls.key", test.certificateErr
			}, func(string, string, string) error { return test.serveErr }, func(string) error { return test.serveErr }, ":8443", ":8000", func(secure bool, err error) {
				fatalCalled, reported, initialized = true, err, secure
			})
			if fatalCalled != test.wantFatal {
				t.Fatalf("fatal callback called = %t, want %t", fatalCalled, test.wantFatal)
			}
			if test.wantFatal && (!errors.Is(reported, test.certificateErr) && !errors.Is(reported, test.serveErr) || initialized != test.wantSecure) {
				t.Errorf("fatal callback received initialized=%t error=%v", initialized, reported)
			}
		})
	}
}

func TestDefaultWebStartupDependencies(t *testing.T) {
	kubeconfig := filepath.Join(t.TempDir(), "kubeconfig")
	config := "apiVersion: v1\nkind: Config\nclusters:\n- name: test\n  cluster:\n    server: https://kubernetes.example.test\nusers:\n- name: test\n  user:\n    token: unit-test-token\ncontexts:\n- name: test\n  context:\n    cluster: test\n    user: test\ncurrent-context: test\n"
	if err := os.WriteFile(kubeconfig, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	setWebMainFlag(t, "kubeconfig", kubeconfig)
	initializeWebClient()
	setWebMainFlag(t, "default-cert-dir", t.TempDir())
	setWebMainFlag(t, "auto-generate-certificates", "false")
	setWebMainFlag(t, "tls-cert-file", "")
	setWebMainFlag(t, "tls-key-file", "")
	certPath, keyPath, err := getWebCertificatePaths()
	if err != nil || certPath != "" || keyPath != "" {
		t.Fatalf("getWebCertificatePaths() = %q, %q, %v; want no configured certificate", certPath, keyPath, err)
	}
}

func setWebMainFlag(t *testing.T, name, value string) {
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
