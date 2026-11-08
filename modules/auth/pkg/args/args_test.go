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

package args

import (
	"testing"

	"github.com/spf13/pflag"
)

func TestArgumentAccessors(t *testing.T) {
	setAuthFlag(t, "address", "127.0.0.1")
	setAuthFlag(t, "port", "8123")
	setAuthFlag(t, "kubeconfig", "/tmp/dashboard-kubeconfig")
	setAuthFlag(t, "apiserver-host", "https://kubernetes.example.test")
	setAuthFlag(t, "apiserver-skip-tls-verify", "true")
	setAuthFlag(t, "apiserver-ca-bundle", "/tmp/ca.pem")
	setAuthFlag(t, "oidc-issuer-url", " https://identity.example.test/ ")
	setAuthFlag(t, "oidc-client-id", " dashboard ")
	setAuthFlag(t, "oidc-client-secret", "secret")
	setAuthFlag(t, "oidc-redirect-url", " https://dashboard.example.test/api/v1/oidc/callback ")
	setAuthFlag(t, "oidc-scopes", "openid groups profile")
	setAuthFlag(t, "oidc-groups-claim", " memberOf ")
	setAuthFlag(t, "oidc-picture-claim", " avatar ")
	setAuthFlag(t, "session-cookie-secure", "false")
	setAuthFlag(t, "anonymous-admin", "true")

	if Address() != "127.0.0.1:8123" || KubeconfigPath() != "/tmp/dashboard-kubeconfig" {
		t.Errorf("address/kubeconfig = %q/%q", Address(), KubeconfigPath())
	}
	if ApiServerHost() != "https://kubernetes.example.test" || !ApiServerSkipTLSVerify() || ApiServerCaBundle() != "/tmp/ca.pem" {
		t.Errorf("API server args = %q, %t, %q", ApiServerHost(), ApiServerSkipTLSVerify(), ApiServerCaBundle())
	}
	if OIDCIssuerURL() != "https://identity.example.test/" || OIDCClientID() != "dashboard" || OIDCClientSecret() != "secret" {
		t.Errorf("OIDC issuer/client = %q/%q/%q", OIDCIssuerURL(), OIDCClientID(), OIDCClientSecret())
	}
	if OIDCRedirectURL() != "https://dashboard.example.test/api/v1/oidc/callback" {
		t.Errorf("OIDCRedirectURL() = %q", OIDCRedirectURL())
	}
	if got := OIDCScopeList(); len(got) != 3 || got[1] != "groups" {
		t.Errorf("OIDCScopeList() = %v", got)
	}
	if OIDCGroupsClaim() != "memberOf" || OIDCPictureClaim() != "avatar" || SessionCookieSecure() {
		t.Errorf("OIDC claims/session cookie = %q/%q/%t", OIDCGroupsClaim(), OIDCPictureClaim(), SessionCookieSecure())
	}
	if !AnonymousAdminEnabled() {
		t.Error("AnonymousAdminEnabled() = false, want true")
	}
}

func TestEnvironmentArgumentHelpers(t *testing.T) {
	t.Setenv("AUTH_TEST_VALUE", " configured ")
	t.Setenv("AUTH_TEST_BOOL", "false")
	if got := envOr("AUTH_TEST_VALUE", "fallback"); got != " configured " {
		t.Errorf("envOr() = %q", got)
	}
	if got := envOr("AUTH_TEST_MISSING", "fallback"); got != "fallback" {
		t.Errorf("envOr(missing) = %q", got)
	}
	if got := envBool("AUTH_TEST_BOOL", true); got {
		t.Error("envBool(false) returned true")
	}
	if got := envBool("AUTH_TEST_MISSING", true); !got {
		t.Error("envBool(missing) did not use fallback")
	}
	t.Setenv("AUTH_TEST_INVALID_BOOL", "sometimes")
	if got := envBool("AUTH_TEST_INVALID_BOOL", true); !got {
		t.Error("envBool(invalid) did not use fallback")
	}
}

func setAuthFlag(t *testing.T, name, value string) {
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
