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

package v1

// OIDCConfig describes the OIDC provider configuration exposed to the frontend.
type OIDCConfig struct {
	// Enabled indicates whether OIDC authentication is configured.
	Enabled bool `json:"enabled"`
	// ProviderName is the display name of the OIDC provider (e.g. "Keycloak", "Okta").
	ProviderName string `json:"providerName,omitempty"`
	// ProviderURL is the issuer URL of the OIDC provider.
	ProviderURL string `json:"providerUrl,omitempty"`
	// ClientID is the OIDC client ID.
	ClientID string `json:"clientId,omitempty"`
	// Scopes are the OIDC scopes requested (e.g., "openid profile email groups").
	Scopes string `json:"scopes,omitempty"`
}

// OIDCLoginResponse is sent after initiating the OIDC login flow.
type OIDCLoginResponse struct {
	// RedirectURL is the URL the client should redirect to for authentication.
	RedirectURL string `json:"redirectUrl,omitempty"`
	// Token is set when OIDC login completes successfully (set as cookie).
	Token string `json:"token,omitempty"`
}

// OIDCCallbackRequest is received after the OIDC provider redirects back.
type OIDCCallbackRequest struct {
	// Code is the authorization code from the OIDC provider.
	Code string `json:"code"`
	// State is the state parameter for CSRF protection.
	State string `json:"state"`
}

// OIDCUserInfo holds the extracted OIDC user identity used for impersonation.
type OIDCUserInfo struct {
	// Username is the Kubernetes username to impersonate (from OIDC "sub" or "email" claim).
	Username string `json:"username"`
	// Groups are the Kubernetes groups to impersonate (from OIDC "groups" claim).
	Groups []string `json:"groups,omitempty"`
}

// OIDCSession is returned after successful OIDC authentication.
// It contains the user info needed for impersonation-based K8s API access.
type OIDCSession struct {
	// Token is the ID token (for reference, not used for K8s API auth in impersonation mode).
	Token string `json:"token,omitempty"`
	// User is the extracted user identity for impersonation.
	User OIDCUserInfo `json:"user"`
}
