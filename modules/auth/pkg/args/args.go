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

package args

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/pflag"
	"k8s.io/klog/v2"

	clientargs "k8s.io/dashboard/client/args"
	"k8s.io/dashboard/csrf"
)

var (
	argPort                   = pflag.Int("port", 8000, "The port for auth service to listen on.")
	argAddress                = pflag.IP("address", net.IPv4(0, 0, 0, 0), "The IP address for auth service to serve on. Set 0.0.0.0 for listening on all interfaces.")
	argKubeconfig             = pflag.String("kubeconfig", "", "path to kubeconfig file")
	argApiServerHost          = pflag.String("apiserver-host", "", "address of the Kubernetes API server to connect to in the format of protocol://address:port, leave it empty if the binary runs inside cluster for local discovery attempt")
	argApiServerSkipTLSVerify = pflag.Bool("apiserver-skip-tls-verify", false, "enable if connection with remote Kubernetes API server should skip TLS verify")
	argApiServerCaBundle      = pflag.String("apiserver-ca-bundle", "", "file containing the x509 certificates used for HTTPS connection to the API Server")
	argOIDCIssuerURL          = pflag.String("oidc-issuer-url", os.Getenv("OIDC_ISSUER_URL"), "OpenID Connect issuer URL; discovery must be available over HTTPS")
	argOIDCClientID           = pflag.String("oidc-client-id", os.Getenv("OIDC_CLIENT_ID"), "OpenID Connect client ID registered with the issuer")
	argOIDCClientSecret       = pflag.String("oidc-client-secret", os.Getenv("OIDC_CLIENT_SECRET"), "OpenID Connect confidential client secret")
	argOIDCRedirectURL        = pflag.String("oidc-redirect-url", os.Getenv("OIDC_REDIRECT_URL"), "exact HTTPS callback URL registered with the issuer")
	argOIDCScopes             = pflag.String("oidc-scopes", envOr("OIDC_SCOPES", "openid profile email"), "space-separated scopes requested from the OpenID Connect issuer")
	argOIDCGroupsClaim        = pflag.String("oidc-groups-claim", envOr("OIDC_GROUPS_CLAIM", "groups"), "ID token or UserInfo claim containing group names")
	argOIDCPictureClaim       = pflag.String("oidc-picture-claim", envOr("OIDC_PICTURE_CLAIM", "picture"), "ID token or UserInfo claim containing the profile picture URL")
	argSessionCookieSecure    = pflag.Bool("session-cookie-secure", envBool("DASHBOARD_SESSION_COOKIE_SECURE", true), "include Secure on browser session cookies; disable only for local HTTP development")
)

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envBool(name string, fallback bool) bool {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func init() {
	// Init klog
	fs := flag.NewFlagSet("", flag.PanicOnError)
	klog.InitFlags(fs)

	// Default log level to 1
	_ = fs.Set("v", "1")

	pflag.CommandLine.AddGoFlagSet(fs)
	pflag.Parse()

	csrf.Ensure()
}

func KubeconfigPath() string {
	return *argKubeconfig
}

func ApiServerHost() string {
	return *argApiServerHost
}

func ApiServerSkipTLSVerify() bool {
	return *argApiServerSkipTLSVerify
}

func ApiServerCaBundle() string {
	return *argApiServerCaBundle
}

func OIDCIssuerURL() string { return strings.TrimSpace(*argOIDCIssuerURL) }

func OIDCClientID() string { return strings.TrimSpace(*argOIDCClientID) }

func OIDCClientSecret() string { return *argOIDCClientSecret }

func OIDCRedirectURL() string { return strings.TrimSpace(*argOIDCRedirectURL) }

func OIDCScopeList() []string {
	return strings.Fields(*argOIDCScopes)
}

func OIDCGroupsClaim() string { return strings.TrimSpace(*argOIDCGroupsClaim) }

func OIDCPictureClaim() string { return strings.TrimSpace(*argOIDCPictureClaim) }

func SessionCookieSecure() bool { return *argSessionCookieSecure }

func AnonymousAdminEnabled() bool { return clientargs.AnonymousAdminEnabled() }

func Address() string {
	return fmt.Sprintf("%s:%d", *argAddress, *argPort)
}
