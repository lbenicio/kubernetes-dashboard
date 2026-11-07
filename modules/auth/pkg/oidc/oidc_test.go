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
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"golang.org/x/oauth2"

	"k8s.io/dashboard/client"
)

type oidcRoundTripFunc func(*http.Request) (*http.Response, error)

func (in oidcRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return in(request)
}

func setManagerForTest(t *testing.T, value *Manager) {
	t.Helper()
	managerMu.Lock()
	previous := manager
	manager = value
	managerMu.Unlock()
	t.Cleanup(func() {
		managerMu.Lock()
		manager = previous
		managerMu.Unlock()
	})
}

func testManager(responseMode string) *Manager {
	return &Manager{
		oauthConfig: oauth2.Config{
			ClientID:    "dashboard-test-client",
			RedirectURL: "https://dashboard.example.test/api/v1/oidc/callback",
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://identity.example.test/authorize",
				TokenURL: "https://identity.example.test/token",
			},
			Scopes: []string{"openid", "profile"},
		},
		clientID:     "dashboard-test-client",
		groupsClaim:  "groups",
		pictureClaim: "picture",
		scopes:       []string{"openid", "profile"},
		responseMode: responseMode,
		httpClient:   http.DefaultClient,
	}
}

func configureOIDCTestSessionKey(t *testing.T) {
	t.Helper()
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("o", 32)))
	if err := client.ConfigureSessionKey(key); err != nil {
		t.Fatalf("ConfigureSessionKey() error = %v", err)
	}
}

func TestBeginLoginBuildsPKCERequestAndProtectedState(t *testing.T) {
	configureOIDCTestSessionKey(t)
	client.SetSessionCookieSecure(true)
	t.Cleanup(func() { client.SetSessionCookieSecure(true) })
	setManagerForTest(t, testManager("query"))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "https://dashboard.example.test/api/v1/oidc/login", nil)
	BeginLogin(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("BeginLogin() status = %d, want %d", response.Code, http.StatusOK)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", response.Header().Get("Cache-Control"))
	}

	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode authorization response: %v", err)
	}
	authorizationURL, err := url.Parse(body["url"])
	if err != nil {
		t.Fatalf("parse authorization URL: %v", err)
	}
	query := authorizationURL.Query()
	if query.Get("response_type") != "code" || query.Get("client_id") != "dashboard-test-client" {
		t.Fatalf("authorization request is missing code flow parameters: %v", query)
	}
	if query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") == "" {
		t.Fatalf("authorization request is missing an S256 PKCE challenge: %v", query)
	}
	if query.Get("state") == "" || query.Get("nonce") == "" {
		t.Fatalf("authorization request is missing state or nonce: %v", query)
	}

	var stateCookie *http.Cookie
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == stateCookieName {
			stateCookie = cookie
			break
		}
	}
	if stateCookie == nil || !stateCookie.HttpOnly || !stateCookie.Secure || stateCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("OIDC state cookie has incorrect security attributes: %#v", stateCookie)
	}
	statePayload, err := client.OpenSessionData(statePurpose, stateCookie.Value)
	if err != nil {
		t.Fatalf("decrypt OIDC state: %v", err)
	}
	var state authorizationState
	if err := json.Unmarshal(statePayload, &state); err != nil {
		t.Fatalf("decode OIDC state: %v", err)
	}
	if state.State != query.Get("state") || state.Nonce != query.Get("nonce") || state.ResponseMode != "query" {
		t.Fatalf("protected state does not match the authorization request: %#v", state)
	}
	verifierHash := sha256.Sum256([]byte(state.Verifier))
	wantChallenge := base64.RawURLEncoding.EncodeToString(verifierHash[:])
	if query.Get("code_challenge") != wantChallenge {
		t.Fatalf("PKCE challenge = %q, want %q", query.Get("code_challenge"), wantChallenge)
	}
}

func TestBeginLoginUsesFormPostWhenConfigured(t *testing.T) {
	configureOIDCTestSessionKey(t)
	client.SetSessionCookieSecure(true)
	t.Cleanup(func() { client.SetSessionCookieSecure(true) })
	setManagerForTest(t, testManager("form_post"))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "https://dashboard.example.test/api/v1/oidc/login", nil)
	BeginLogin(response, request)
	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := url.Parse(body["url"])
	if err != nil {
		t.Fatal(err)
	}
	if got := authorizationURL.Query().Get("response_mode"); got != "form_post" {
		t.Fatalf("response_mode = %q, want form_post", got)
	}
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == stateCookieName && cookie.SameSite != http.SameSiteNoneMode {
			t.Fatalf("form_post state cookie SameSite = %v, want None", cookie.SameSite)
		}
	}
}

func TestBeginLoginReturnsNotFoundWhenDisabled(t *testing.T) {
	setManagerForTest(t, nil)
	response := httptest.NewRecorder()
	BeginLogin(response, httptest.NewRequest(http.MethodPost, "/api/v1/oidc/login", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("BeginLogin() status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestCompleteLoginRejectsInvalidCallbacks(t *testing.T) {
	configureOIDCTestSessionKey(t)
	tests := []struct {
		name       string
		method     string
		query      string
		body       string
		content    string
		state      *authorizationState
		cookieData string
	}{
		{name: "provider error", method: http.MethodGet, query: "error=access_denied", state: nil},
		{name: "missing state cookie", method: http.MethodGet, query: "state=s&code=c"},
		{name: "wrong state", method: http.MethodGet, query: "state=other&code=c", state: &authorizationState{State: "expected", Nonce: "n", Verifier: "v", ResponseMode: "query", CreatedAt: time.Now()}},
		{name: "expired state", method: http.MethodGet, query: "state=s&code=c", state: &authorizationState{State: "s", Nonce: "n", Verifier: "v", ResponseMode: "query", CreatedAt: time.Now().Add(-stateLifetime - time.Second)}},
		{name: "future state", method: http.MethodGet, query: "state=s&code=c", state: &authorizationState{State: "s", Nonce: "n", Verifier: "v", ResponseMode: "query", CreatedAt: time.Now().Add(time.Second)}},
		{name: "wrong response mode", method: http.MethodGet, query: "state=s&code=c", state: &authorizationState{State: "s", Nonce: "n", Verifier: "v", ResponseMode: "form_post", CreatedAt: time.Now()}},
		{name: "state in query for form post", method: http.MethodPost, query: "state=s&code=c", state: &authorizationState{State: "s", Nonce: "n", Verifier: "v", ResponseMode: "query", CreatedAt: time.Now()}, content: "application/x-www-form-urlencoded"},
		{name: "unsupported form content type", method: http.MethodPost, body: "state=s&code=c", content: "text/plain"},
		{name: "oversized form", method: http.MethodPost, body: strings.Repeat("a", (64<<10)+1), content: "application/x-www-form-urlencoded"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manager := testManager("query")
			setManagerForTest(t, manager)
			requestURL := "https://dashboard.example.test/api/v1/oidc/callback"
			if test.query != "" {
				requestURL += "?" + test.query
			}
			request := httptest.NewRequest(test.method, requestURL, strings.NewReader(test.body))
			if test.content != "" {
				request.Header.Set("Content-Type", test.content)
			}
			if test.state != nil {
				payload, err := json.Marshal(test.state)
				if err != nil {
					t.Fatal(err)
				}
				cookieValue, err := client.SealSessionData(statePurpose, payload)
				if err != nil {
					t.Fatal(err)
				}
				request.AddCookie(&http.Cookie{Name: stateCookieName, Value: cookieValue})
			}
			if test.cookieData != "" {
				request.AddCookie(&http.Cookie{Name: stateCookieName, Value: test.cookieData})
			}

			response := httptest.NewRecorder()
			CompleteLogin(response, request)
			if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login?auth_error=oidc" {
				t.Fatalf("CompleteLogin() = %d %q, want redirect to login error", response.Code, response.Header().Get("Location"))
			}
			if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Referrer-Policy") != "no-referrer" {
				t.Errorf("callback response missing privacy headers: %v", response.Header())
			}
		})
	}
}

func TestCompleteLoginReturnsNotFoundWhenDisabled(t *testing.T) {
	setManagerForTest(t, nil)
	response := httptest.NewRecorder()
	CompleteLogin(response, httptest.NewRequest(http.MethodGet, "/api/v1/oidc/callback", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("CompleteLogin() status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestRefreshSessionSkipsUnneededRefreshAndRejectsMissingRefreshToken(t *testing.T) {
	setManagerForTest(t, nil)
	if refreshed, err := RefreshSession(t.Context(), nil); err != nil || refreshed {
		t.Fatalf("RefreshSession(nil) = %t, %v, want false, nil", refreshed, err)
	}

	setManagerForTest(t, testManager("query"))
	nonOIDC := &client.BrowserSession{OIDC: false}
	if refreshed, err := RefreshSession(t.Context(), nonOIDC); err != nil || refreshed {
		t.Fatalf("RefreshSession(non-OIDC) = %t, %v, want false, nil", refreshed, err)
	}
	valid := &client.BrowserSession{OIDC: true, TokenExpiry: time.Now().Add(time.Minute)}
	if refreshed, err := RefreshSession(t.Context(), valid); err != nil || refreshed {
		t.Fatalf("RefreshSession(unexpired) = %t, %v, want false, nil", refreshed, err)
	}
	expired := &client.BrowserSession{OIDC: true, TokenExpiry: time.Now().Add(-time.Second)}
	if refreshed, err := RefreshSession(t.Context(), expired); err == nil || refreshed {
		t.Fatalf("RefreshSession(expired without refresh token) = %t, %v, want error", refreshed, err)
	}
}

func TestApplyClaimsPrefersProfileClaimsAndExtractsGroupsAndPicture(t *testing.T) {
	manager := &Manager{groupsClaim: "groups", pictureClaim: "avatar"}
	session := &client.BrowserSession{Subject: "subject-123"}
	claims := map[string]json.RawMessage{
		"preferred_username": json.RawMessage(`"ada"`),
		"name":               json.RawMessage(`"Ada Lovelace"`),
		"email":              json.RawMessage(`"ada@example.test"`),
		"avatar":             json.RawMessage(`"https://images.example.test/ada.png"`),
		"groups":             json.RawMessage(`["engineering","operators"]`),
	}
	manager.applyClaims(session, claims)
	if session.Name != "ada" || session.Email != "ada@example.test" {
		t.Fatalf("profile claims were not mapped: %#v", session)
	}
	if session.Picture != "https://images.example.test/ada.png" || strings.Join(session.Groups, ",") != "engineering,operators" {
		t.Fatalf("picture or group claims were not mapped: %#v", session)
	}

	preferred := &client.BrowserSession{Name: "Existing", Email: "existing@example.test", Groups: []string{"existing"}, Picture: "https://existing.example.test/avatar.png"}
	manager.applyClaims(preferred, claims)
	if preferred.Name != "Existing" || preferred.Email != "existing@example.test" || len(preferred.Groups) != 1 || preferred.Picture != "https://existing.example.test/avatar.png" {
		t.Fatalf("applyClaims() overwrote existing profile data: %#v", preferred)
	}
}

func TestApplyClaimsUsesFallbacksAndRejectsUnsafePictures(t *testing.T) {
	manager := &Manager{groupsClaim: "groups", pictureClaim: "picture"}
	for _, test := range []struct {
		name   string
		claims map[string]json.RawMessage
		want   string
	}{
		{name: "name", claims: map[string]json.RawMessage{"name": json.RawMessage(`"User Name"`), "picture": json.RawMessage(`"http://images.example.test/a.png"`)}, want: "User Name"},
		{name: "email", claims: map[string]json.RawMessage{"email": json.RawMessage(`"user@example.test"`)}, want: "user@example.test"},
		{name: "subject", claims: map[string]json.RawMessage{}, want: "subject"},
	} {
		t.Run(test.name, func(t *testing.T) {
			session := &client.BrowserSession{Subject: "subject"}
			manager.applyClaims(session, test.claims)
			if session.Name != test.want {
				t.Fatalf("Name = %q, want %q", session.Name, test.want)
			}
			if session.Picture != "" {
				t.Errorf("unsafe picture URL was accepted: %q", session.Picture)
			}
		})
	}

	if got := claimStrings(json.RawMessage(`"cluster-admins"`)); len(got) != 1 || got[0] != "cluster-admins" {
		t.Fatalf("claimStrings() did not accept a scalar group: %v", got)
	}
	if got := claimStrings(json.RawMessage(`true`)); got != nil {
		t.Fatalf("claimStrings() accepted a non-string claim: %v", got)
	}
	if got := claimString(map[string]json.RawMessage{"name": json.RawMessage(`"  Ada  "`)}, "name"); got != "Ada" {
		t.Fatalf("claimString() = %q, want trimmed value", got)
	}
	if got := claimString(map[string]json.RawMessage{"name": json.RawMessage(`1`)}, "name"); got != "" {
		t.Fatalf("claimString() accepted a non-string value: %q", got)
	}
	if got := claimString(map[string]json.RawMessage{}, ""); got != "" {
		t.Fatalf("claimString() accepted an empty claim name: %q", got)
	}
}

func TestTokenScopesUsesGrantedScopesAndCopiesFallback(t *testing.T) {
	fallback := []string{"openid", "profile"}
	if got := tokenScopes(&oauth2.Token{TokenType: "Bearer"}, fallback); strings.Join(got, ",") != "openid,profile" {
		t.Fatalf("tokenScopes() fallback = %v", got)
	} else {
		got[0] = "changed"
		if fallback[0] != "openid" {
			t.Fatal("tokenScopes() returned the caller's fallback slice")
		}
	}
	granted := (&oauth2.Token{}).WithExtra(map[string]any{"scope": "openid groups profile"})
	if got := tokenScopes(granted, fallback); strings.Join(got, ",") != "openid,groups,profile" {
		t.Fatalf("tokenScopes() granted = %v", got)
	}
	if got := tokenScopes(nil, nil); got != nil {
		t.Fatalf("tokenScopes(nil, nil) = %v, want nil", got)
	}
}

func TestValidateEndpointURL(t *testing.T) {
	for _, valid := range []string{
		"https://identity.example.test/issuer",
		"http://localhost:8080/issuer",
		"http://127.0.0.1:8080/issuer",
		"http://[::1]:8080/issuer",
	} {
		if err := validateEndpointURL(valid); err != nil {
			t.Errorf("validateEndpointURL(%q) error = %v", valid, err)
		}
	}
	for _, invalid := range []string{
		"/issuer",
		"http://identity.example.test/issuer",
		"https://user:password@identity.example.test/issuer",
		"https://identity.example.test/issuer#fragment",
		"ftp://identity.example.test/issuer",
	} {
		if err := validateEndpointURL(invalid); err == nil {
			t.Errorf("validateEndpointURL(%q) unexpectedly succeeded", invalid)
		}
	}
}

func TestSafePictureURLAndRandomStateHelpers(t *testing.T) {
	for _, valid := range []string{"https://images.example.test/a.png", "https://images.example.test/a.png?size=small"} {
		if !safePictureURL(valid) {
			t.Errorf("safePictureURL(%q) = false", valid)
		}
	}
	for _, invalid := range []string{"http://images.example.test/a.png", "//images.example.test/a.png", "https://user@images.example.test/a.png", ""} {
		if safePictureURL(invalid) {
			t.Errorf("safePictureURL(%q) = true", invalid)
		}
	}
	first, err := randomValue(32)
	if err != nil {
		t.Fatal(err)
	}
	second, err := randomValue(32)
	if err != nil {
		t.Fatal(err)
	}
	if first == second || len(first) != 43 {
		t.Fatalf("randomValue() returned invalid values: %q, %q", first, second)
	}
	if !constantTimeEquals("state", "state") || constantTimeEquals("state", "other") {
		t.Fatal("constantTimeEquals() returned an incorrect result")
	}
}

func TestRedirectLoginErrorUsesFixedLocation(t *testing.T) {
	response := httptest.NewRecorder()
	redirectLoginError(response, httptest.NewRequest(http.MethodGet, "/api/v1/oidc/callback", nil))
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login?auth_error=oidc" {
		t.Fatalf("redirectLoginError() = %d %q", response.Code, response.Header().Get("Location"))
	}
}

func TestFetchProviderMetadataValidatesDiscoveryDocument(t *testing.T) {
	issuer := "https://identity.example.test/issuer"
	validMetadata := func() map[string]any {
		return map[string]any{
			"issuer":                                issuer,
			"authorization_endpoint":                "https://identity.example.test/authorize",
			"token_endpoint":                        "https://identity.example.test/token",
			"jwks_uri":                              "https://identity.example.test/keys",
			"userinfo_endpoint":                     "https://identity.example.test/userinfo",
			"response_types_supported":              []string{"code"},
			"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
			"token_endpoint_auth_methods_supported": []string{"client_secret_basic"},
		}
	}
	tests := []struct {
		name       string
		status     int
		body       func() any
		wantErr    string
		wantMethod string
	}{
		{name: "valid document", status: http.StatusOK, body: func() any { return validMetadata() }, wantMethod: "client_secret_basic"},
		{name: "non-success response", status: http.StatusBadGateway, body: func() any { return validMetadata() }, wantErr: "discovery returned HTTP 502"},
		{name: "invalid document shape", status: http.StatusOK, body: func() any { return "not an object" }, wantErr: "cannot unmarshal string"},
		{name: "wrong issuer", status: http.StatusOK, body: func() any { value := validMetadata(); value["issuer"] = "https://other.example.test"; return value }, wantErr: "does not exactly match"},
		{name: "missing authorization endpoint", status: http.StatusOK, body: func() any { value := validMetadata(); delete(value, "authorization_endpoint"); return value }, wantErr: "missing authorization_endpoint"},
		{name: "insecure token endpoint", status: http.StatusOK, body: func() any {
			value := validMetadata()
			value["token_endpoint"] = "http://identity.example.test/token"
			return value
		}, wantErr: "invalid token_endpoint"},
		{name: "insecure userinfo endpoint", status: http.StatusOK, body: func() any {
			value := validMetadata()
			value["userinfo_endpoint"] = "http://identity.example.test/userinfo"
			return value
		}, wantErr: "invalid userinfo_endpoint"},
		{name: "authorization code not supported", status: http.StatusOK, body: func() any {
			value := validMetadata()
			value["response_types_supported"] = []string{"token"}
			return value
		}, wantErr: "authorization code response type"},
		{name: "authorization code grant not supported", status: http.StatusOK, body: func() any {
			value := validMetadata()
			value["grant_types_supported"] = []string{"implicit"}
			return value
		}, wantErr: "authorization_code grant"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := json.Marshal(tc.body())
			if err != nil {
				t.Fatal(err)
			}
			client := &http.Client{Transport: oidcRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				if request.Method != http.MethodGet || request.URL.String() != issuer+"/.well-known/openid-configuration" {
					t.Errorf("discovery request = %s %s", request.Method, request.URL)
				}
				return &http.Response{
					StatusCode: tc.status,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(string(encoded))),
					Request:    request,
				}, nil
			})}
			metadata, err := fetchProviderMetadata(context.Background(), client, issuer)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("fetchProviderMetadata() error = %v, want containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || metadata == nil || !slices.Contains(metadata.TokenAuthMethods, tc.wantMethod) {
				t.Fatalf("fetchProviderMetadata() = %#v, %v", metadata, err)
			}
		})
	}
}

func TestFetchProviderMetadataRejectsMissingAndOversizedData(t *testing.T) {
	issuer := "https://identity.example.test"
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "empty response", body: ""},
		{name: "oversized response", body: strings.Repeat(" ", 1<<20) + `{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: oidcRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(tc.body)), Request: request}, nil
			})}
			if _, err := fetchProviderMetadata(context.Background(), client, issuer); err == nil {
				t.Fatal("fetchProviderMetadata() unexpectedly accepted invalid response")
			}
		})
	}
}

func TestFetchProviderMetadataRejectsEveryRequiredEndpoint(t *testing.T) {
	issuer := "https://identity.example.test"
	base := map[string]any{
		"issuer": issuer, "authorization_endpoint": "https://identity.example.test/authorize",
		"token_endpoint": "https://identity.example.test/token", "jwks_uri": "https://identity.example.test/keys",
		"response_types_supported": []string{"code"},
	}
	for _, field := range []string{"authorization_endpoint", "token_endpoint", "jwks_uri"} {
		t.Run(field, func(t *testing.T) {
			metadata := make(map[string]any, len(base))
			for key, value := range base {
				metadata[key] = value
			}
			metadata[field] = ""
			body, err := json.Marshal(metadata)
			if err != nil {
				t.Fatal(err)
			}
			client := &http.Client{Transport: oidcRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body))), Request: request}, nil
			})}
			_, err = fetchProviderMetadata(context.Background(), client, issuer)
			if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("missing %s", field)) {
				t.Fatalf("fetchProviderMetadata() error = %v, want missing %s", err, field)
			}
		})
	}
}

func TestInitializeRejectsInvalidConfigurationBeforeDiscovery(t *testing.T) {
	configureOIDCTestSessionKey(t)
	setManagerForTest(t, nil)
	setOIDCFlag(t, "oidc-issuer-url", "")
	setOIDCFlag(t, "oidc-client-id", "")
	setOIDCFlag(t, "oidc-client-secret", "")
	setOIDCFlag(t, "oidc-redirect-url", "")
	setOIDCFlag(t, "anonymous-admin", "false")
	if err := Initialize(); err != nil {
		t.Fatalf("Initialize() with OIDC disabled = %v", err)
	}

	setOIDCFlag(t, "oidc-issuer-url", "https://identity.example.test")
	if err := Initialize(); err == nil || !strings.Contains(err.Error(), "requires issuer URL, client ID, and redirect URL") {
		t.Fatalf("Initialize() partial configuration error = %v", err)
	}

	setOIDCFlag(t, "anonymous-admin", "true")
	if err := Initialize(); err == nil || !strings.Contains(err.Error(), "cannot be enabled at the same time") {
		t.Fatalf("Initialize() anonymous mode conflict error = %v", err)
	}
	setOIDCFlag(t, "anonymous-admin", "false")
	setOIDCFlag(t, "oidc-client-id", "dashboard")
	setOIDCFlag(t, "oidc-redirect-url", "https://dashboard.example.test/wrong-path")
	if err := Initialize(); err == nil || !strings.Contains(err.Error(), "callback path") {
		t.Fatalf("Initialize() callback path error = %v", err)
	}

	setOIDCFlag(t, "oidc-redirect-url", "https://dashboard.example.test/api/v1/oidc/callback")
	setOIDCFlag(t, "oidc-issuer-url", "http://identity.example.test")
	if err := Initialize(); err == nil || !strings.Contains(err.Error(), "invalid OIDC issuer URL") {
		t.Fatalf("Initialize() insecure issuer error = %v", err)
	}
}

func setOIDCFlag(t *testing.T, name, value string) {
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
