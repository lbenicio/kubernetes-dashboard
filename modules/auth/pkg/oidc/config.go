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

package oidc

// Config holds the OIDC provider configuration.
type Config struct {
	// IssuerURL is the OIDC provider's issuer URL (e.g., https://accounts.google.com).
	IssuerURL string
	// ClientID is the OIDC client ID registered with the provider.
	ClientID string
	// ClientSecret is the OIDC client secret registered with the provider.
	ClientSecret string
	// RedirectURL is the callback URL where the OIDC provider sends the authorization code.
	RedirectURL string
	// Scopes are the OIDC scopes to request (space-separated). Defaults to "openid profile email groups".
	Scopes string
	// CookieSecret is used to encrypt session cookies. Must be at least 32 bytes.
	CookieSecret string
	// ProviderName is a human-readable name for the OIDC provider.
	ProviderName string
}

// IsEnabled returns true if the OIDC provider is configured.
func (c *Config) IsEnabled() bool {
	return c != nil && c.IssuerURL != "" && c.ClientID != "" && c.ClientSecret != ""
}

// DefaultScopes returns the default OIDC scopes if none are configured.
func (c *Config) DefaultScopes() string {
	if c.Scopes == "" {
		return "openid profile email groups"
	}
	return c.Scopes
}
