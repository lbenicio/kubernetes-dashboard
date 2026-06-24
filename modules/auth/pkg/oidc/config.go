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
	// InsecureSkipVerify skips TLS certificate verification for the OIDC provider.
	InsecureSkipVerify bool
	// CABundle is a path to a CA bundle for OIDC provider TLS verification.
	CABundle string
	// UsernameClaim is the OIDC claim used for the Kubernetes username (default: "email").
	UsernameClaim string
	// GroupsClaim is the OIDC claim used for Kubernetes groups (default: "groups").
	GroupsClaim string
	// AvatarClaim is the OIDC claim used for the avatar URL (default: "picture").
	AvatarClaim string
	// NameClaim is the OIDC claim used for the display name (default: "name").
	NameClaim string
	// EmailClaim is the OIDC claim used for the email address (default: "email").
	EmailClaim string
	// AllowedGroup is an optional group required for access. If empty, all authenticated users are allowed.
	AllowedGroup string
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

func (c *Config) usernameClaim() string {
	if c.UsernameClaim != "" {
		return c.UsernameClaim
	}
	return "email"
}

func (c *Config) groupsClaim() string {
	if c.GroupsClaim != "" {
		return c.GroupsClaim
	}
	return "groups"
}

func (c *Config) avatarClaim() string {
	if c.AvatarClaim != "" {
		return c.AvatarClaim
	}
	return "picture"
}

func (c *Config) nameClaim() string {
	if c.NameClaim != "" {
		return c.NameClaim
	}
	return "name"
}

func (c *Config) emailClaim() string {
	if c.EmailClaim != "" {
		return c.EmailClaim
	}
	return "email"
}
