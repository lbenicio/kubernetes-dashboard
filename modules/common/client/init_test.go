// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

package client

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	apierrors "k8s.io/dashboard/errors"
)

func TestConfigBuilderOptionsAndDefaults(t *testing.T) {
	builder := newConfigBuilder(
		WithUserAgent("coverage-test"),
		WithMasterUrl("https://kubernetes.example.test"),
		WithKubeconfig("test-kubeconfig"),
		WithInsecureTLSSkipVerify(true),
		WithCaBundle("ca.pem"),
		WithSessionEncryptionKey("session-key"),
	)
	if builder.userAgent != "coverage-test" || builder.masterUrl != "https://kubernetes.example.test" || builder.kubeconfigPath != "test-kubeconfig" || !builder.insecure || builder.caBundlePath != "ca.pem" || builder.sessionKey != "session-key" {
		t.Fatalf("newConfigBuilder() = %#v", builder)
	}
	if builder.setConfigDefaults(nil) != nil || setConfigRateLimitDefaults(nil) != nil {
		t.Fatal("default helpers should preserve nil configs")
	}

	config, err := newConfigBuilder(
		WithUserAgent("coverage-test"),
		WithMasterUrl("https://kubernetes.example.test"),
		WithInsecureTLSSkipVerify(true),
		WithCaBundle("ca.pem"),
	).buildBaseConfig()
	if err != nil {
		t.Fatalf("buildBaseConfig() error = %v", err)
	}
	if config.Host != "https://kubernetes.example.test" || config.UserAgent != "dashboard/coverage-test" || config.ContentType != DefaultContentType || !config.Insecure || config.CAFile != "ca.pem" || config.QPS != DefaultQPS || config.Burst != DefaultBurst {
		t.Fatalf("buildBaseConfig() = %#v", config)
	}
}

func TestConfigBuilderReturnsMalformedKubeconfigErrorWithCABundle(t *testing.T) {
	kubeconfig := filepath.Join(t.TempDir(), "invalid-kubeconfig")
	if err := os.WriteFile(kubeconfig, []byte(": invalid yaml"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := newConfigBuilder(WithKubeconfig(kubeconfig), WithCaBundle("ca.pem")).buildBaseConfig()
	if err == nil {
		t.Fatal("buildBaseConfig() accepted malformed kubeconfig")
	}
}

func TestBuildAuthInfoAndImpersonation(t *testing.T) {
	request := httptest.NewRequest("GET", "http://dashboard.test", nil)
	if _, err := buildAuthInfo(request); !apierrors.IsUnauthorized(err) {
		t.Fatalf("buildAuthInfo() without auth = %v, want unauthorized", err)
	}
	SetAuthorizationHeader(request, "access-token")
	request.Header.Add(ImpersonateUserHeader, "ada")
	request.Header.Add(ImpersonateGroupHeader, "admins")
	request.Header.Add(ImpersonateGroupHeader, "operators")
	request.Header[ImpersonateUserExtraHeader+"scopes"] = []string{"read", "write"}

	authInfo, err := buildAuthInfo(request)
	if err != nil {
		t.Fatalf("buildAuthInfo() error = %v", err)
	}
	if authInfo.Token != "access-token" || authInfo.Impersonate != "ada" || len(authInfo.ImpersonateGroups) != 2 || len(authInfo.ImpersonateUserExtra["scopes"]) != 2 {
		t.Fatalf("buildAuthInfo() = %#v", authInfo)
	}

	withoutUser := httptest.NewRequest("GET", "http://dashboard.test", nil)
	withoutUser.Header.Add(ImpersonateGroupHeader, "ignored")
	info := &clientcmdapi.AuthInfo{ImpersonateUserExtra: map[string][]string{}}
	handleImpersonation(info, withoutUser)
	if info.Impersonate != "" || len(info.ImpersonateGroups) != 0 {
		t.Fatalf("impersonation without a user should be ignored: %#v", info)
	}
}

func TestConfigFromRequestUsesTokenAndAnonymousBaseConfig(t *testing.T) {
	previous := baseConfig
	baseConfig = &rest.Config{Host: "https://kubernetes.example.test", Insecure: true}
	t.Cleanup(func() { baseConfig = previous })

	request := httptest.NewRequest("GET", "http://dashboard.test", nil)
	SetAuthorizationHeader(request, "access-token")
	config, err := configFromRequest(request)
	if err != nil || config.Host != baseConfig.Host || config.BearerToken != "access-token" || !config.Insecure {
		t.Fatalf("configFromRequest() = %#v, %v", config, err)
	}

	flag := pflag.Lookup("anonymous-admin")
	old := flag.Value.String()
	t.Cleanup(func() { _ = flag.Value.Set(old) })
	if err := flag.Value.Set("true"); err != nil {
		t.Fatal(err)
	}
	anonymous, err := configFromRequest(httptest.NewRequest("GET", "http://dashboard.test", nil))
	if err != nil || anonymous == baseConfig || anonymous.Host != baseConfig.Host {
		t.Fatalf("anonymous configFromRequest() = %#v, %v", anonymous, err)
	}
}

func TestUninitializedClientHelpers(t *testing.T) {
	previousConfig, previousClient := baseConfig, inClusterClient
	baseConfig, inClusterClient = nil, nil
	t.Cleanup(func() {
		baseConfig, inClusterClient = previousConfig, previousClient
	})
	request := httptest.NewRequest("GET", "http://dashboard.test", nil)
	if isInitialized() || InClusterClient() != nil {
		t.Fatal("client should remain uninitialized")
	}
	if _, err := Client(request); err == nil {
		t.Fatal("Client() succeeded before initialization")
	}
	if _, err := APIExtensionsClient(request); err == nil {
		t.Fatal("APIExtensionsClient() succeeded before initialization")
	}
	if _, err := Config(request); err == nil {
		t.Fatal("Config() succeeded before initialization")
	}
}
