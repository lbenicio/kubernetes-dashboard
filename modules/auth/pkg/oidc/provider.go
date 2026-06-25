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
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"k8s.io/klog/v2"
)

// Provider handles OIDC authentication using the standard go-oidc library.
type Provider struct {
	config    *Config
	session   *SessionManager
	provider  *oidc.Provider
	oauth2Cfg *oauth2.Config
	verifier  *oidc.IDTokenVerifier
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

	// Use custom HTTP client for self-signed/internal CAs
	hc := p.httpClient()
	ctx = oidc.ClientContext(ctx, hc)
	oauth2Ctx := context.WithValue(ctx, oauth2.HTTPClient, hc)

	provider, err := oidc.NewProvider(ctx, p.config.IssuerURL)
	if err != nil {
		return fmt.Errorf("OIDC discovery failed: %w", err)
	}
	p.provider = provider

	p.oauth2Cfg = &oauth2.Config{
		ClientID:     p.config.ClientID,
		ClientSecret: p.config.ClientSecret,
		RedirectURL:  p.config.RedirectURL,
		Scopes:       p.parseScopes(),
		Endpoint:     provider.Endpoint(),
	}

	// Create ID token verifier
	p.verifier = provider.Verifier(&oidc.Config{
		ClientID: p.config.ClientID,
	})

	// Link session manager with oauth2 config for token refresh
	p.session.SetOAuth2Config(p.oauth2Cfg)

	_ = oauth2Ctx // ensure custom HTTP client context is captured

	klog.InfoS("OIDC provider initialized",
		"issuer", p.config.IssuerURL,
		"clientId", p.config.ClientID,
	)
	return nil
}

// GetConfig returns the provider configuration.
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

// AuthCodeURL generates the authorization URL with PKCE, state, and nonce.
// Returns the URL, PKCE verifier, state, and nonce.
func (p *Provider) AuthCodeURL() (string, string, string, string, error) {
	if p.oauth2Cfg == nil {
		return "", "", "", "", fmt.Errorf("OIDC provider not initialized")
	}

	state, err := GenerateState()
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to generate state: %w", err)
	}

	nonce, err := GenerateNonce()
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	verifier := oauth2.GenerateVerifier()

	opts := []oauth2.AuthCodeOption{
		oauth2.S256ChallengeOption(verifier),
		oidc.Nonce(nonce),
	}

	urlStr := p.oauth2Cfg.AuthCodeURL(state, opts...)

	return urlStr, verifier, state, nonce, nil
}

// Exchange exchanges the authorization code for tokens using PKCE.
func (p *Provider) Exchange(ctx context.Context, code string, verifier string) (*oauth2.Token, error) {
	if p.oauth2Cfg == nil {
		return nil, fmt.Errorf("OIDC provider not initialized")
	}

	// Inject custom HTTP client for self-signed certs
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.httpClient())

	token, err := p.oauth2Cfg.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	return token, nil
}

// ExtractIDToken extracts the raw ID token string from the OAuth2 token.
func (p *Provider) ExtractIDToken(token *oauth2.Token) (string, error) {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return "", fmt.Errorf("no id_token in token response")
	}
	return rawIDToken, nil
}

// ValidateAndExtractUser verifies the ID token using the OIDC verifier and extracts user claims.
func (p *Provider) ValidateAndExtractUser(ctx context.Context, rawIDToken string, nonce string) (*OIDCUserInfo, error) {
	if p.verifier == nil {
		return nil, fmt.Errorf("OIDC verifier not initialized")
	}

	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("ID token verification failed: %w", err)
	}

	// Verify nonce if provided (skip for token refresh where original nonce is unavailable)
	if nonce != "" && idToken.Nonce != nonce {
		return nil, fmt.Errorf("ID token nonce mismatch")
	}

	// Extract claims into our user info struct
	var claims map[string]interface{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to parse ID token claims: %w", err)
	}

	userInfo := extractUserInfoFromClaims(claims, p.config)

	klog.InfoS("OIDC user authenticated",
		"username", userInfo.Username,
		"groups", userInfo.Groups,
		"email", userInfo.Email,
	)

	return userInfo, nil
}

// CreateSession validates the ID token, extracts user info, and sets cookies.
func (p *Provider) CreateSession(w http.ResponseWriter, r *http.Request, token *oauth2.Token, nonce, state string) (*OIDCUserInfo, error) {
	rawIDToken, err := p.ExtractIDToken(token)
	if err != nil {
		return nil, err
	}

	userInfo, err := p.ValidateAndExtractUser(r.Context(), rawIDToken, nonce)
	if err != nil {
		return nil, fmt.Errorf("failed to validate ID token: %w", err)
	}

	expiry := token.Expiry
	if expiry.IsZero() {
		expiry = time.Now().Add(1 * time.Hour)
	}

	sessionData := &SessionData{
		IDToken:      rawIDToken,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		Expiry:       expiry,
		State:        state,
		Nonce:        nonce,
		Username:     userInfo.Username,
		Groups:       userInfo.Groups,
		DisplayName:  userInfo.DisplayName,
		Email:        userInfo.Email,
		AvatarURL:    userInfo.AvatarURL,
	}

	if err := p.session.SetSessionCookie(w, sessionData); err != nil {
		return nil, fmt.Errorf("failed to set session cookie: %w", err)
	}

	p.session.SetTokenCookie(w, rawIDToken, expiry)
	p.session.ClearStateCookie(w)

	return userInfo, nil
}

// httpClient returns an HTTP client configured with TLS settings.
func (p *Provider) httpClient() *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: p.config.InsecureSkipVerify,
		},
	}

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

// parseScopes parses space-separated scopes, ensuring "openid" is always included.
func (p *Provider) parseScopes() []string {
	scopesStr := p.config.DefaultScopes()
	if scopesStr == "" {
		return []string{oidc.ScopeOpenID, "profile", "email", "groups"}
	}
	scopes := strings.Fields(scopesStr)
	// Ensure openid is present
	hasOpenID := false
	for _, s := range scopes {
		if s == oidc.ScopeOpenID {
			hasOpenID = true
			break
		}
	}
	if !hasOpenID {
		scopes = append([]string{oidc.ScopeOpenID}, scopes...)
	}
	return scopes
}
