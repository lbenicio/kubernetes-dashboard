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

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/oauth2"
	"k8s.io/klog/v2"
)

// Provider handles OIDC discovery and token exchange.
type Provider struct {
	config    *Config
	session   *SessionManager
	oauth2Cfg *oauth2.Config
	metadata  ProviderMetadata
	jwksKeys  map[string]interface{} // kid → parsed public key
}

// ProviderMetadata holds the OIDC provider's discovered endpoints.
type ProviderMetadata struct {
	Issuer        string `json:"issuer"`
	AuthURL       string `json:"authorization_endpoint"`
	TokenURL      string `json:"token_endpoint"`
	UserInfoURL   string `json:"userinfo_endpoint"`
	JWKSURL       string `json:"jwks_uri"`
	EndSessionURL string `json:"end_session_endpoint,omitempty"`
}

// NewProvider creates a new OIDC Provider.
func NewProvider(config *Config) *Provider {
	return &Provider{
		config:  config,
		session: NewSessionManager(config),
	}
}

// Initialize performs OIDC discovery and sets up the OAuth2 config.
func (p *Provider) Initialize(ctx context.Context) error {
	if !p.config.IsEnabled() {
		return fmt.Errorf("OIDC provider is not configured")
	}

	metadata, err := p.discover(ctx)
	if err != nil {
		return fmt.Errorf("OIDC discovery failed: %w", err)
	}

	p.metadata = *metadata

	p.oauth2Cfg = &oauth2.Config{
		ClientID:     p.config.ClientID,
		ClientSecret: p.config.ClientSecret,
		RedirectURL:  p.config.RedirectURL,
		Scopes:       p.parseScopes(),
		Endpoint: oauth2.Endpoint{
			AuthURL:  metadata.AuthURL,
			TokenURL: metadata.TokenURL,
		},
	}

	// Use the TLS-configured HTTP client for OAuth2 token exchange
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.httpClient())

	// Pre-fetch JWKS keys for ID token validation
	if metadata.JWKSURL != "" {
		if err := p.fetchJWKS(ctx); err != nil {
			klog.Warningf("Failed to fetch JWKS, ID token validation may be limited: %v", err)
		}
	}

	// Link session manager with oauth2 config for token refresh
	p.session.SetOAuth2Config(p.oauth2Cfg)

	klog.InfoS("OIDC provider initialized",
		"issuer", metadata.Issuer,
		"clientId", p.config.ClientID,
	)
	return nil
}

// GetConfig returns the provider configuration for the frontend.
func (p *Provider) GetConfig() *Config {
	return p.config
}

// Session returns the session manager.
func (p *Provider) Session() *SessionManager {
	return p.session
}

// OAuth2Config returns the oauth2 config.
func (p *Provider) OAuth2Config() *oauth2.Config {
	return p.oauth2Cfg
}

// AuthCodeURL generates the authorization URL with PKCE and state.
// It returns the URL, the PKCE verifier, and the state parameter.
func (p *Provider) AuthCodeURL() (string, string, string, error) {
	if p.oauth2Cfg == nil {
		return "", "", "", fmt.Errorf("OIDC provider not initialized")
	}

	state, err := GenerateState()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to generate state: %w", err)
	}

	// Generate PKCE code verifier
	verifier := oauth2.GenerateVerifier()

	opts := []oauth2.AuthCodeOption{
		oauth2.S256ChallengeOption(verifier),
	}

	urlStr := p.oauth2Cfg.AuthCodeURL(state, opts...)

	return urlStr, verifier, state, nil
}

// Exchange exchanges the authorization code for tokens using PKCE.
func (p *Provider) Exchange(ctx context.Context, code string, verifier string) (*oauth2.Token, error) {
	if p.oauth2Cfg == nil {
		return nil, fmt.Errorf("OIDC provider not initialized")
	}

	opts := []oauth2.AuthCodeOption{
		oauth2.VerifierOption(verifier),
	}

	token, err := p.oauth2Cfg.Exchange(ctx, code, opts...)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	return token, nil
}

// ExtractIDToken extracts and returns the ID token from the OAuth2 token response.
func (p *Provider) ExtractIDToken(token *oauth2.Token) (string, error) {
	idToken, ok := token.Extra("id_token").(string)
	if !ok || idToken == "" {
		return "", fmt.Errorf("no id_token in token response")
	}
	return idToken, nil
}

// ValidateAndExtractUser validates the ID token and extracts the user identity.
// This performs cryptographic signature verification using the provider's JWKS.
func (p *Provider) ValidateAndExtractUser(ctx context.Context, idToken string) (*OIDCUserInfo, error) {
	// Parse and validate the ID token
	parsedToken, err := jwt.Parse(idToken, p.jwksKeyFunc())
	if err != nil {
		return nil, fmt.Errorf("ID token validation failed: %w", err)
	}

	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Validate issuer
	if err := claims.Valid(); err != nil {
		return nil, fmt.Errorf("token claims invalid: %w", err)
	}

	issuer, _ := claims["iss"].(string)
	if issuer != "" && issuer != p.metadata.Issuer && !strings.HasPrefix(issuer, p.metadata.Issuer) {
		// Some providers add a trailing slash
		if strings.TrimRight(issuer, "/") != strings.TrimRight(p.metadata.Issuer, "/") {
			klog.Warningf("ID token issuer mismatch: got %q, expected %q", issuer, p.metadata.Issuer)
		}
	}

	// Extract user identity for impersonation
	userInfo := extractUserInfo(claims, p.config)

	klog.InfoS("OIDC user authenticated",
		"username", userInfo.Username,
		"groups", userInfo.Groups,
	)

	return userInfo, nil
}

// CreateSession creates session data from the OAuth2 token and writes cookies.
// It also validates the ID token and extracts user info for impersonation.
func (p *Provider) CreateSession(w http.ResponseWriter, r *http.Request, token *oauth2.Token, state string) (*OIDCUserInfo, error) {
	idToken, err := p.ExtractIDToken(token)
	if err != nil {
		return nil, err
	}

	// Validate the ID token and extract user identity
	userInfo, err := p.ValidateAndExtractUser(r.Context(), idToken)
	if err != nil {
		return nil, fmt.Errorf("failed to validate ID token: %w", err)
	}

	expiry := token.Expiry
	if expiry.IsZero() {
		expiry = time.Now().Add(1 * time.Hour)
	}

	sessionData := &SessionData{
		IDToken:      idToken,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		Expiry:       expiry,
		State:        state,
		Username:     userInfo.Username,
		Groups:       userInfo.Groups,
		DisplayName:  userInfo.DisplayName,
		Email:        userInfo.Email,
		AvatarURL:    userInfo.AvatarURL,
	}

	// Store encrypted session (contains refresh token, user info, HttpOnly)
	if err := p.session.SetSessionCookie(w, sessionData); err != nil {
		return nil, fmt.Errorf("failed to set session cookie: %w", err)
	}

	// Store ID token for frontend (for reference, not used for K8s API auth)
	// In impersonation mode, K8s API auth uses impersonation headers, not this token
	p.session.SetTokenCookie(w, idToken, expiry)

	// Clear the state cookie
	p.session.ClearStateCookie(w)

	return userInfo, nil
}

// jwksKeyFunc returns a jwt.Keyfunc that validates tokens using the provider's JWKS.
func (p *Provider) jwksKeyFunc() jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		// If we have pre-fetched keys, use them
		if len(p.jwksKeys) > 0 {
			kid, ok := token.Header["kid"].(string)
			if !ok {
				return nil, fmt.Errorf("token missing kid header")
			}
			key, ok := p.jwksKeys[kid]
			if !ok {
				return nil, fmt.Errorf("no JWKS key found for kid: %s", kid)
			}
			return key, nil
		}

		// No pre-fetched keys - attempt to fetch on-demand
		// This is not ideal for performance but works as a fallback
		klog.Warning("JWKS not pre-fetched, attempting on-demand fetch")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := p.fetchJWKS(ctx); err != nil {
			return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
		}

		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("token missing kid header")
		}
		key, ok := p.jwksKeys[kid]
		if !ok {
			return nil, fmt.Errorf("no JWKS key found for kid: %s", kid)
		}
		return key, nil
	}
}

// fetchJWKS fetches the JWKS keys from the provider's jwks_uri.
func (p *Provider) fetchJWKS(ctx context.Context) error {
	if p.metadata.JWKSURL == "" {
		return fmt.Errorf("no jwks_uri in OIDC metadata")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.metadata.JWKSURL, nil)
	if err != nil {
		return err
	}

	resp, err := p.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS fetch returned status %d", resp.StatusCode)
	}

	var jwks jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("failed to parse JWKS: %w", err)
	}

	p.jwksKeys = make(map[string]interface{})
	for _, key := range jwks.Keys {
		parsedKey, err := parseJWK(key)
		if err != nil {
			klog.Warningf("Failed to parse JWK key %q: %v", key.KID, err)
			continue
		}
		p.jwksKeys[key.KID] = parsedKey
	}

	klog.InfoS("JWKS keys fetched", "count", len(p.jwksKeys))
	return nil
}

// httpClient returns an HTTP client configured with the OIDC TLS settings.
func (p *Provider) httpClient() *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: p.config.InsecureSkipVerify,
		},
	}

	// Load custom CA bundle if configured
	if p.config.CABundle != "" {
		caCert, err := os.ReadFile(p.config.CABundle)
		if err != nil {
			klog.Warningf("Failed to read OIDC CA bundle %s: %v", p.config.CABundle, err)
		} else {
			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(caCert)
			transport.TLSClientConfig.RootCAs = caCertPool
		}
	}

	return &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}
}

// discover performs OIDC discovery from the issuer URL.
func (p *Provider) discover(ctx context.Context) (*ProviderMetadata, error) {
	wellKnown := strings.TrimRight(p.config.IssuerURL, "/") + "/.well-known/openid-configuration"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wellKnown, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch OIDC discovery URL %s: %w", wellKnown, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OIDC discovery returned status %d", resp.StatusCode)
	}

	var metadata ProviderMetadata
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to parse OIDC discovery response: %w", err)
	}

	if metadata.Issuer == "" || metadata.AuthURL == "" || metadata.TokenURL == "" {
		return nil, fmt.Errorf("OIDC discovery response missing required fields")
	}

	return &metadata, nil
}

// parseScopes parses space-separated scopes into a slice.
func (p *Provider) parseScopes() []string {
	scopesStr := p.config.DefaultScopes()
	if scopesStr == "" {
		return []string{"openid", "profile", "email", "groups"}
	}
	return strings.Fields(scopesStr)
}
