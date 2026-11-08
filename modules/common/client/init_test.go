// Copyright 2017 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

package client

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	apierrors "k8s.io/dashboard/errors"
)

func TestConfigBuilderOptionsAndDefaults(t *testing.T) {
	transportWrapper := func(next http.RoundTripper) http.RoundTripper { return next }
	builder := newConfigBuilder(
		WithUserAgent("coverage-test"),
		WithMasterUrl("https://kubernetes.example.test"),
		WithKubeconfig("test-kubeconfig"),
		WithInsecureTLSSkipVerify(true),
		WithCaBundle("ca.pem"),
		WithSessionEncryptionKey("session-key"),
		WithWrapTransport(transportWrapper),
	)
	if builder.userAgent != "coverage-test" || builder.masterUrl != "https://kubernetes.example.test" || builder.kubeconfigPath != "test-kubeconfig" || !builder.insecure || builder.caBundlePath != "ca.pem" || builder.sessionKey != "session-key" || builder.wrapTransport == nil {
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
		WithWrapTransport(transportWrapper),
	).buildBaseConfig()
	if err != nil {
		t.Fatalf("buildBaseConfig() error = %v", err)
	}
	if config.Host != "https://kubernetes.example.test" || config.UserAgent != "dashboard/coverage-test" || config.ContentType != DefaultContentType || !config.Insecure || config.CAFile != "ca.pem" || config.QPS != DefaultQPS || config.Burst != DefaultBurst || config.WrapTransport == nil {
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

func TestConfigBuilderReturnsInClusterConfigErrorWhenUnavailable(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")

	if config, err := newConfigBuilder().buildBaseConfig(); err == nil || config != nil {
		t.Fatalf("buildBaseConfig() = %#v, %v; want an in-cluster config error", config, err)
	}
}

func TestInitStoresConfiguredBaseClientConfig(t *testing.T) {
	previousConfig := baseConfig
	baseConfig = nil
	t.Cleanup(func() { baseConfig = previousConfig })

	clusterContextFlag := pflag.Lookup("cluster-context-enabled")
	previousClusterContext := clusterContextFlag.Value.String()
	if err := clusterContextFlag.Value.Set("false"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = clusterContextFlag.Value.Set(previousClusterContext) })
	anonymousFlag := pflag.Lookup("anonymous-admin")
	previousAnonymous := anonymousFlag.Value.String()
	if err := anonymousFlag.Value.Set("true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = anonymousFlag.Value.Set(previousAnonymous) })
	previousSessionKey := append([]byte(nil), sessionKey...)
	t.Cleanup(func() {
		sessionKeyMu.Lock()
		sessionKey = previousSessionKey
		sessionKeyMu.Unlock()
	})
	sessionKeyMu.Lock()
	sessionKey = nil
	sessionKeyMu.Unlock()
	sessionKey := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("s", 32)))

	Init(WithMasterUrl("https://kubernetes.example.test"), WithUserAgent("init-test"), WithSessionEncryptionKey(sessionKey))
	if baseConfig == nil || baseConfig.Host != "https://kubernetes.example.test" || baseConfig.UserAgent != DefaultUserAgent+"/init-test" {
		t.Fatalf("Init() baseConfig = %#v", baseConfig)
	}
	if !HasSessionKey() {
		t.Fatal("Init() did not configure the supplied browser session key")
	}
}

func TestInitReadsSessionKeyFromArgsWhenOptionIsOmitted(t *testing.T) {
	previousConfig := baseConfig
	baseConfig = nil
	t.Cleanup(func() { baseConfig = previousConfig })

	clusterContextFlag := pflag.Lookup("cluster-context-enabled")
	previousClusterContext := clusterContextFlag.Value.String()
	if err := clusterContextFlag.Value.Set("false"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = clusterContextFlag.Value.Set(previousClusterContext) })
	anonymousFlag := pflag.Lookup("anonymous-admin")
	previousAnonymous := anonymousFlag.Value.String()
	if err := anonymousFlag.Value.Set("false"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = anonymousFlag.Value.Set(previousAnonymous) })

	encodedKey := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
	sessionKeyFlag := pflag.Lookup("session-key")
	previousSessionKeyFlag := sessionKeyFlag.Value.String()
	if err := sessionKeyFlag.Value.Set(encodedKey); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sessionKeyFlag.Value.Set(previousSessionKeyFlag) })
	previousSessionKey := append([]byte(nil), sessionKey...)
	t.Cleanup(func() {
		sessionKeyMu.Lock()
		sessionKey = previousSessionKey
		sessionKeyMu.Unlock()
	})

	Init(WithMasterUrl("https://kubernetes.example.test"))
	if baseConfig == nil || !HasSessionKey() {
		t.Fatalf("Init() baseConfig=%#v sessionKeyConfigured=%t", baseConfig, HasSessionKey())
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
	baseConfig = &rest.Config{
		Host:          "https://kubernetes.example.test",
		Insecure:      true,
		WrapTransport: func(next http.RoundTripper) http.RoundTripper { return next },
	}
	t.Cleanup(func() { baseConfig = previous })

	request := httptest.NewRequest("GET", "http://dashboard.test", nil)
	SetAuthorizationHeader(request, "access-token")
	config, err := configFromRequest(request)
	if err != nil || config.Host != baseConfig.Host || config.BearerToken != "access-token" || !config.Insecure || config.WrapTransport == nil {
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
