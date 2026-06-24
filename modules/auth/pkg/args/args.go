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

	"github.com/spf13/pflag"
	"k8s.io/klog/v2"

	"k8s.io/dashboard/csrf"
)

var (
	argPort                   = pflag.Int("port", 8000, "The port for auth service to listen on.")
	argAddress                = pflag.IP("address", net.IPv4(0, 0, 0, 0), "The IP address for auth service to serve on. Set 0.0.0.0 for listening on all interfaces.")
	argKubeconfig             = pflag.String("kubeconfig", "", "path to kubeconfig file")
	argApiServerHost          = pflag.String("apiserver-host", "", "address of the Kubernetes API server to connect to in the format of protocol://address:port, leave it empty if the binary runs inside cluster for local discovery attempt")
	argApiServerSkipTLSVerify = pflag.Bool("apiserver-skip-tls-verify", false, "enable if connection with remote Kubernetes API server should skip TLS verify")
	argApiServerCaBundle      = pflag.String("apiserver-ca-bundle", "", "file containing the x509 certificates used for HTTPS connection to the API Server")

	// OIDC configuration flags
	argOIDCIssuerURL    = pflag.String("oidc-issuer-url", "", "The URL of the OIDC provider (e.g., https://accounts.google.com). Enables OIDC authentication when set.")
	argOIDCClientID     = pflag.String("oidc-client-id", "", "The OIDC client ID registered with the provider.")
	argOIDCClientSecret = pflag.String("oidc-client-secret", "", "The OIDC client secret registered with the provider.")
	argOIDCRedirectURL  = pflag.String("oidc-redirect-url", "", "The OIDC redirect/callback URL. If empty, it is auto-generated from the request.")
	argOIDCScopes       = pflag.String("oidc-scopes", "openid profile email groups", "The OIDC scopes to request (space-separated).")
	argOIDCCookieSecret = pflag.String("oidc-cookie-secret", "", "A secret key used to encrypt OIDC session cookies. Must be at least 32 bytes. Auto-generated if empty.")
	argOIDCProviderName = pflag.String("oidc-provider-name", "", "A human-readable name for the OIDC provider displayed on the login page.")
	argOIDCSkipTLSVerify = pflag.Bool("oidc-skip-tls-verify", false, "Skip TLS certificate verification for OIDC provider connections.")
	argOIDCCABundle     = pflag.String("oidc-ca-bundle", "", "Path to a CA bundle file for OIDC provider TLS verification.")
	argOIDCUsernameClaim = pflag.String("oidc-username-claim", "email", "The OIDC claim to use as the Kubernetes username (e.g., email, sub, name).")
	argOIDCGroupsClaim   = pflag.String("oidc-groups-claim", "groups", "The OIDC claim to use for Kubernetes groups.")
	argOIDCAvatarClaim   = pflag.String("oidc-avatar-claim", "picture", "The OIDC claim to use for the user avatar URL.")
	argOIDCNameClaim     = pflag.String("oidc-name-claim", "name", "The OIDC claim to use for the display name.")
	argOIDCEmailClaim    = pflag.String("oidc-email-claim", "email", "The OIDC claim to use for the email address.")
	argOIDCAllowedGroup  = pflag.String("oidc-allowed-group", "", "If set, only users in this group are allowed to access the dashboard.")
)

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

func Address() string {
	return fmt.Sprintf("%s:%d", *argAddress, *argPort)
}

// OIDC configuration getters

func OIDCIssuerURL() string {
	return *argOIDCIssuerURL
}

func OIDCClientID() string {
	return *argOIDCClientID
}

func OIDCClientSecret() string {
	if secret := os.Getenv("OIDC_CLIENT_SECRET"); secret != "" {
		return secret
	}
	return *argOIDCClientSecret
}

func OIDCRedirectURL() string {
	return *argOIDCRedirectURL
}

func OIDCScopes() string {
	return *argOIDCScopes
}

func OIDCCookieSecret() string {
	return *argOIDCCookieSecret
}

func OIDCProviderName() string {
	return *argOIDCProviderName
}

func OIDCSkipTLSVerify() bool {
	return *argOIDCSkipTLSVerify
}

func OIDCCABundle() string {
	return *argOIDCCABundle
}

func OIDCUsernameClaim() string {
	return *argOIDCUsernameClaim
}

func OIDCGroupsClaim() string {
	return *argOIDCGroupsClaim
}

func OIDCAvatarClaim() string {
	return *argOIDCAvatarClaim
}

func OIDCNameClaim() string {
	return *argOIDCNameClaim
}

func OIDCEmailClaim() string {
	return *argOIDCEmailClaim
}

func OIDCAllowedGroup() string {
	return *argOIDCAllowedGroup
}
