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
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package oidc

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"k8s.io/dashboard/auth/pkg/args"
	"k8s.io/dashboard/client"
)

const (
	stateCookieName = "kd-oidc-state"
	statePurpose    = "kubernetes-dashboard-oidc-state-v1"
	stateLifetime   = 5 * time.Minute
	sessionLifetime = 8 * time.Hour
	refreshWindow   = 30 * time.Second
)

type authorizationState struct {
	State        string    `json:"state"`
	Nonce        string    `json:"nonce"`
	Verifier     string    `json:"verifier"`
	ResponseMode string    `json:"responseMode"`
	CreatedAt    time.Time `json:"createdAt"`
}

type providerMetadata struct {
	oidc.ProviderConfig
	ResponseTypesSupported []string `json:"response_types_supported"`
	ResponseModesSupported []string `json:"response_modes_supported"`
	GrantTypesSupported    []string `json:"grant_types_supported"`
	TokenAuthMethods       []string `json:"token_endpoint_auth_methods_supported"`
}

type Manager struct {
	provider     *oidc.Provider
	verifier     *oidc.IDTokenVerifier
	oauthConfig  oauth2.Config
	httpClient   *http.Client
	clientID     string
	groupsClaim  string
	pictureClaim string
	scopes       []string
	responseMode string
}

type oidcSettings struct {
	issuerURL    string
	clientID     string
	clientSecret string
	redirectURL  string
}

var (
	managerMu sync.RWMutex
	manager   *Manager
)

// Initialize discovers and validates the configured issuer. It returns an
// error for partial or insecure configuration instead of silently falling
// back to token authentication.
func Initialize() error {
	settings, configured, err := configuredOIDCSettings()
	if err != nil {
		return err
	}
	if !configured {
		return nil
	}
	instance, err := buildManager(settings)
	if err != nil {
		return err
	}
	managerMu.Lock()
	manager = instance
	managerMu.Unlock()
	return nil
}

func configuredOIDCSettings() (oidcSettings, bool, error) {
	settings := oidcSettings{
		issuerURL:    args.OIDCIssuerURL(),
		clientID:     args.OIDCClientID(),
		clientSecret: args.OIDCClientSecret(),
		redirectURL:  args.OIDCRedirectURL(),
	}
	configured := settings.issuerURL != "" || settings.clientID != "" || settings.clientSecret != "" || settings.redirectURL != ""
	if args.AnonymousAdminEnabled() && configured {
		return oidcSettings{}, false, errors.New("anonymous admin mode and OIDC cannot be enabled at the same time")
	}
	if !configured {
		return settings, false, nil
	}
	if err := validateOIDCSettings(settings); err != nil {
		return oidcSettings{}, false, err
	}
	return settings, true, nil
}

func validateOIDCSettings(settings oidcSettings) error {
	if settings.issuerURL == "" || settings.clientID == "" || settings.redirectURL == "" {
		return errors.New("OIDC requires issuer URL, client ID, and redirect URL")
	}
	if !client.HasSessionKey() {
		return errors.New("OIDC requires DASHBOARD_SESSION_KEY to be shared by the Auth and API services")
	}
	if err := validateEndpointURL(settings.issuerURL); err != nil {
		return fmt.Errorf("invalid OIDC issuer URL: %w", err)
	}
	if err := validateEndpointURL(settings.redirectURL); err != nil {
		return fmt.Errorf("invalid OIDC redirect URL: %w", err)
	}
	callback, err := url.Parse(settings.redirectURL)
	if err != nil || callback.Path != "/api/v1/oidc/callback" || callback.RawQuery != "" {
		return errors.New("OIDC redirect URL must use the /api/v1/oidc/callback path without a query string")
	}
	return nil
}

func buildManager(settings oidcSettings) (*Manager, error) {
	httpClient := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)
	metadata, err := fetchProviderMetadata(ctx, httpClient, settings.issuerURL)
	if err != nil {
		return nil, fmt.Errorf("validate OIDC discovery metadata: %w", err)
	}
	provider := metadata.NewProvider(ctx)
	config, scopes, responseMode, err := buildOAuthConfig(settings, metadata, provider)
	if err != nil {
		return nil, err
	}
	return &Manager{
		provider:     provider,
		verifier:     provider.VerifierContext(context.WithValue(context.Background(), oauth2.HTTPClient, httpClient), &oidc.Config{ClientID: settings.clientID}),
		oauthConfig:  config,
		httpClient:   httpClient,
		clientID:     settings.clientID,
		groupsClaim:  args.OIDCGroupsClaim(),
		pictureClaim: args.OIDCPictureClaim(),
		scopes:       scopes,
		responseMode: responseMode,
	}, nil
}

func buildOAuthConfig(settings oidcSettings, metadata *providerMetadata, provider *oidc.Provider) (oauth2.Config, []string, string, error) {
	scopes := args.OIDCScopeList()
	if !slices.Contains(scopes, oidc.ScopeOpenID) {
		scopes = append([]string{oidc.ScopeOpenID}, scopes...)
	}
	endpoint := provider.Endpoint()
	if err := configureTokenAuth(&endpoint, settings.clientSecret, metadata.TokenAuthMethods); err != nil {
		return oauth2.Config{}, nil, "", err
	}
	if args.OIDCGroupsClaim() == "" || args.OIDCPictureClaim() == "" {
		return oauth2.Config{}, nil, "", errors.New("OIDC groups and picture claim names must not be empty")
	}
	config := oauth2.Config{
		ClientID:     settings.clientID,
		ClientSecret: settings.clientSecret,
		Endpoint:     endpoint,
		RedirectURL:  settings.redirectURL,
		Scopes:       scopes,
	}
	return config, scopes, responseMode(metadata.ResponseModesSupported), nil
}

func configureTokenAuth(endpoint *oauth2.Endpoint, clientSecret string, methods []string) error {
	if clientSecret == "" {
		if len(methods) > 0 && !slices.Contains(methods, "none") {
			return errors.New("OIDC issuer does not advertise public-client token authentication")
		}
		endpoint.AuthStyle = oauth2.AuthStyleInParams
		return nil
	}
	switch {
	case len(methods) == 0 || slices.Contains(methods, "client_secret_basic"):
		endpoint.AuthStyle = oauth2.AuthStyleInHeader
	case slices.Contains(methods, "client_secret_post"):
		endpoint.AuthStyle = oauth2.AuthStyleInParams
	default:
		return errors.New("OIDC issuer does not advertise a supported confidential-client token authentication method")
	}
	return nil
}

func responseMode(supportedModes []string) string {
	if args.SessionCookieSecure() && slices.Contains(supportedModes, "form_post") {
		return "form_post"
	}
	return "query"
}

func Enabled() bool {
	managerMu.RLock()
	defer managerMu.RUnlock()
	return manager != nil
}

func BeginLogin(w http.ResponseWriter, r *http.Request) {
	instance := current()
	if instance == nil {
		http.NotFound(w, r)
		return
	}

	state := authorizationState{CreatedAt: time.Now().UTC(), ResponseMode: instance.responseMode}
	var err error
	if state.State, err = randomValue(32); err != nil {
		http.Error(w, "Could not start sign in", http.StatusInternalServerError)
		return
	}
	if state.Nonce, err = randomValue(32); err != nil {
		http.Error(w, "Could not start sign in", http.StatusInternalServerError)
		return
	}
	state.Verifier = oauth2.GenerateVerifier()
	encoded, err := json.Marshal(state)
	if err != nil {
		http.Error(w, "Could not start sign in", http.StatusInternalServerError)
		return
	}
	protected, err := client.SealSessionData(statePurpose, encoded)
	if err != nil {
		http.Error(w, "Could not start sign in", http.StatusInternalServerError)
		return
	}
	sameSite := http.SameSiteLaxMode
	if instance.responseMode == "form_post" {
		sameSite = http.SameSiteNoneMode
	}
	client.WriteProtectedCookieWithSameSite(w, r, stateCookieName, protected, int(stateLifetime.Seconds()), sameSite)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	options := []oauth2.AuthCodeOption{oauth2.S256ChallengeOption(state.Verifier), oidc.Nonce(state.Nonce)}
	if instance.responseMode == "form_post" {
		options = append(options, oauth2.SetAuthURLParam("response_mode", "form_post"))
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"url": instance.oauthConfig.AuthCodeURL(state.State, options...)})
}

func CompleteLogin(w http.ResponseWriter, r *http.Request) {
	instance := current()
	if instance == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	client.ClearProtectedCookie(w, r, stateCookieName)
	callbackValues, valid := callbackValues(w, r)
	if !valid || callbackValues.Get("error") != "" {
		redirectLoginError(w, r)
		return
	}

	state, valid := authorizationStateFromCallback(r, callbackValues, instance)
	if !valid {
		redirectLoginError(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, oauth2.HTTPClient, instance.httpClient)
	session, err := exchangeCallback(ctx, instance, callbackValues, state)
	if err != nil {
		redirectLoginError(w, r)
		return
	}
	if err := client.SetBrowserSessionCookie(w, r, session); err != nil {
		http.Error(w, "Could not complete sign in", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func callbackValues(w http.ResponseWriter, r *http.Request) (url.Values, bool) {
	if r.Method != http.MethodPost {
		return r.URL.Query(), true
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	mediaType, _, mediaTypeErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err := r.ParseForm(); err != nil || mediaTypeErr != nil || mediaType != "application/x-www-form-urlencoded" {
		return nil, false
	}
	return r.PostForm, true
}

func authorizationStateFromCallback(r *http.Request, callbackValues url.Values, instance *Manager) (authorizationState, bool) {
	stateCookie, err := r.Cookie(stateCookieName)
	if err != nil {
		return authorizationState{}, false
	}
	statePayload, err := client.OpenSessionData(statePurpose, stateCookie.Value)
	if err != nil {
		return authorizationState{}, false
	}
	var state authorizationState
	if json.Unmarshal(statePayload, &state) != nil || state.State == "" || state.Nonce == "" || state.Verifier == "" ||
		state.ResponseMode != instance.responseMode ||
		(state.ResponseMode == "form_post" && r.Method != http.MethodPost) ||
		(state.ResponseMode == "query" && r.Method != http.MethodGet) ||
		time.Since(state.CreatedAt) > stateLifetime || time.Since(state.CreatedAt) < 0 ||
		!constantTimeEquals(state.State, callbackValues.Get("state")) {
		return authorizationState{}, false
	}
	return state, true
}

func exchangeCallback(ctx context.Context, instance *Manager, callbackValues url.Values, state authorizationState) (client.BrowserSession, error) {
	token, err := instance.oauthConfig.Exchange(ctx, callbackValues.Get("code"), oauth2.VerifierOption(state.Verifier))
	if err != nil {
		return client.BrowserSession{}, err
	}
	if token.AccessToken == "" || !strings.EqualFold(token.TokenType, "Bearer") {
		return client.BrowserSession{}, errors.New("OIDC token response did not contain a bearer access token")
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return client.BrowserSession{}, errors.New("OIDC token response did not contain an ID token")
	}
	idToken, err := instance.verifyIDToken(ctx, rawIDToken, token.AccessToken)
	if err != nil {
		return client.BrowserSession{}, err
	}
	if !constantTimeEquals(idToken.Nonce, state.Nonce) {
		return client.BrowserSession{}, errors.New("OIDC ID token nonce did not match the authorization request")
	}
	return instance.newBrowserSession(ctx, token, rawIDToken, idToken)
}

func RefreshSession(ctx context.Context, session *client.BrowserSession) (bool, error) {
	instance := current()
	if instance == nil || session == nil || !session.OIDC {
		return false, nil
	}
	if session.TokenExpiry.After(time.Now().Add(refreshWindow)) {
		return false, nil
	}
	if session.RefreshToken == "" {
		return false, errors.New("OIDC session expired")
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, oauth2.HTTPClient, instance.httpClient)
	oldSubject := session.Subject
	previousToken := &oauth2.Token{
		AccessToken:  session.AccessToken,
		RefreshToken: session.RefreshToken,
		TokenType:    "Bearer",
		Expiry:       time.Now(),
	}
	refreshed, err := instance.oauthConfig.TokenSource(ctx, previousToken).Token()
	if err != nil {
		return false, err
	}
	rawIDToken, ok := refreshed.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return false, errors.New("OIDC issuer did not return an ID token during refresh")
	}
	idToken, err := instance.verifyIDToken(ctx, rawIDToken, refreshed.AccessToken)
	if err != nil || idToken.Subject != oldSubject {
		return false, errors.New("OIDC refresh returned an invalid identity token")
	}

	session.Groups = nil
	session.KubernetesToken = rawIDToken
	session.Subject = idToken.Subject
	session.AccessToken = refreshed.AccessToken
	session.RefreshToken = refreshed.RefreshToken
	if session.RefreshToken == "" {
		session.RefreshToken = previousToken.RefreshToken
	}
	session.TokenExpiry = idToken.Expiry
	if tokenScopes := tokenScopes(refreshed, instance.scopes); len(tokenScopes) > 0 {
		session.Scopes = tokenScopes
	}
	instance.applyIDTokenClaims(session, idToken)
	instance.applyUserInfo(ctx, session, refreshed)
	return true, nil
}

func (m *Manager) newBrowserSession(ctx context.Context, token *oauth2.Token, rawIDToken string, idToken *oidc.IDToken) (client.BrowserSession, error) {
	session := client.BrowserSession{
		KubernetesToken: rawIDToken,
		OIDC:            true,
		Subject:         idToken.Subject,
		AccessToken:     token.AccessToken,
		RefreshToken:    token.RefreshToken,
		TokenExpiry:     idToken.Expiry,
		ExpiresAt:       time.Now().Add(sessionLifetime),
		Scopes:          tokenScopes(token, m.scopes),
	}
	m.applyIDTokenClaims(&session, idToken)
	m.applyUserInfo(ctx, &session, token)
	return session, nil
}

func (m *Manager) applyIDTokenClaims(session *client.BrowserSession, idToken *oidc.IDToken) {
	var claims map[string]json.RawMessage
	if idToken.Claims(&claims) != nil {
		return
	}
	m.applyClaims(session, claims)
}

func (m *Manager) verifyIDToken(ctx context.Context, rawIDToken, accessToken string) (*oidc.IDToken, error) {
	idToken, err := m.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, err
	}
	if idToken.Subject == "" || idToken.IssuedAt.IsZero() || idToken.IssuedAt.After(time.Now().Add(5*time.Minute)) {
		return nil, errors.New("OIDC ID token has invalid subject or issued-at claim")
	}
	var claims map[string]json.RawMessage
	if err := idToken.Claims(&claims); err != nil {
		return nil, err
	}
	var authorizedParty string
	if raw := claims["azp"]; len(raw) > 0 && json.Unmarshal(raw, &authorizedParty) != nil {
		return nil, errors.New("OIDC ID token has an invalid authorized-party claim")
	}
	if (len(idToken.Audience) > 1 && authorizedParty == "") || (authorizedParty != "" && authorizedParty != m.clientID) {
		return nil, errors.New("OIDC ID token authorized-party claim does not match this client")
	}
	if idToken.AccessTokenHash != "" {
		if accessToken == "" || idToken.VerifyAccessToken(accessToken) != nil {
			return nil, errors.New("OIDC ID token access-token hash does not match")
		}
	}
	return idToken, nil
}

func (m *Manager) applyUserInfo(ctx context.Context, session *client.BrowserSession, token *oauth2.Token) {
	userInfo, err := m.provider.UserInfo(ctx, oauth2.StaticTokenSource(token))
	if err != nil || userInfo.Subject == "" || userInfo.Subject != session.Subject {
		return
	}
	var claims map[string]json.RawMessage
	if userInfo.Claims(&claims) == nil {
		m.applyClaims(session, claims)
	}
}

func (m *Manager) applyClaims(session *client.BrowserSession, claims map[string]json.RawMessage) {
	if session.Name == "" {
		session.Name = claimString(claims, "preferred_username")
	}
	if session.Name == "" {
		session.Name = claimString(claims, "name")
	}
	if session.Email == "" {
		session.Email = claimString(claims, "email")
	}
	if session.Name == "" {
		session.Name = session.Email
	}
	if session.Name == "" {
		session.Name = session.Subject
	}
	if session.Picture == "" {
		picture := claimString(claims, m.pictureClaim)
		if safePictureURL(picture) {
			session.Picture = picture
		}
	}
	if len(session.Groups) == 0 {
		session.Groups = claimStrings(claims[m.groupsClaim])
	}
}

func tokenScopes(token *oauth2.Token, fallback []string) []string {
	if token != nil {
		if scope, ok := token.Extra("scope").(string); ok && strings.TrimSpace(scope) != "" {
			return strings.Fields(scope)
		}
	}
	return slices.Clone(fallback)
}

func current() *Manager {
	managerMu.RLock()
	defer managerMu.RUnlock()
	return manager
}

func validateEndpointURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" {
		return errors.New("must be an absolute URL without user information or fragments")
	}
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme == "http" {
		host := u.Hostname()
		if host == "localhost" || host == "127.0.0.1" || host == "::1" {
			return nil
		}
	}
	return errors.New("must use HTTPS except for a loopback development URL")
}

func fetchProviderMetadata(ctx context.Context, httpClient *http.Client, issuer string) (*providerMetadata, error) {
	requestURL := strings.TrimSuffix(issuer, "/") + "/.well-known/openid-configuration"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery returned HTTP %d", response.StatusCode)
	}
	var metadata providerMetadata
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&metadata); err != nil {
		return nil, err
	}
	if metadata.IssuerURL != issuer {
		return nil, errors.New("discovery issuer does not exactly match configured issuer")
	}
	for name, endpoint := range map[string]string{
		"authorization_endpoint": metadata.AuthURL,
		"token_endpoint":         metadata.TokenURL,
		"jwks_uri":               metadata.JWKSURL,
	} {
		if endpoint == "" {
			return nil, fmt.Errorf("discovery is missing %s", name)
		}
		if err := validateEndpointURL(endpoint); err != nil {
			return nil, fmt.Errorf("invalid %s: %w", name, err)
		}
	}
	if metadata.UserInfoURL != "" {
		if err := validateEndpointURL(metadata.UserInfoURL); err != nil {
			return nil, fmt.Errorf("invalid userinfo_endpoint: %w", err)
		}
	}
	if !slices.Contains(metadata.ResponseTypesSupported, "code") {
		return nil, errors.New("issuer does not advertise the authorization code response type")
	}
	if len(metadata.GrantTypesSupported) > 0 && !slices.Contains(metadata.GrantTypesSupported, "authorization_code") {
		return nil, errors.New("issuer does not advertise the authorization_code grant")
	}
	return &metadata, nil
}

func safePictureURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil
}

func claimString(claims map[string]json.RawMessage, name string) string {
	var value string
	if name == "" || json.Unmarshal(claims[name], &value) != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func claimStrings(raw json.RawMessage) []string {
	var values []string
	if json.Unmarshal(raw, &values) == nil {
		return values
	}
	var value string
	if json.Unmarshal(raw, &value) == nil && value != "" {
		return []string{value}
	}
	return nil
}

func randomValue(size int) (string, error) {
	value := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func constantTimeEquals(left, right string) bool {
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func redirectLoginError(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/login?auth_error=oidc", http.StatusSeeOther)
}
