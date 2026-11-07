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

package oidc

import (
	"crypto"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"k8s.io/dashboard/client"
)

type protocolFixture struct {
	issuer        string
	responseModes []string
	key           *rsa.PrivateKey
	mu            sync.RWMutex
	state         authorizationState
	tokenStatus   int
	tokenType     string
	omitIDToken   bool
	nonceOverride string
}

func newProtocolFixture(t *testing.T, responseModes []string) *protocolFixture {
	t.Helper()
	key, err := rsa.GenerateKey(cryptorand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate OIDC signing key: %v", err)
	}
	fixture := &protocolFixture{issuer: "https://identity.example.test", responseModes: responseModes, key: key}
	return fixture
}

func (in *protocolFixture) roundTrip(request *http.Request) (*http.Response, error) {
	var (
		status = http.StatusOK
		body   string
	)
	switch request.URL.Path {
	case "/.well-known/openid-configuration":
		body = in.json(map[string]any{
			"issuer":                                in.issuer,
			"authorization_endpoint":                in.issuer + "/authorize",
			"token_endpoint":                        in.issuer + "/token",
			"jwks_uri":                              in.issuer + "/keys",
			"userinfo_endpoint":                     in.issuer + "/userinfo",
			"response_types_supported":              []string{"code"},
			"response_modes_supported":              in.responseModes,
			"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
			"token_endpoint_auth_methods_supported": []string{"client_secret_post"},
		})
	case "/keys":
		body = in.json(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "alg": "RS256", "use": "sig", "kid": "dashboard-test-key",
			"n": base64.RawURLEncoding.EncodeToString(in.key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(in.key.E)).Bytes()),
		}}})
	case "/token":
		status, body = in.tokenResponse(request)
	case "/userinfo":
		if request.Header.Get("Authorization") != "Bearer access-token-one" && request.Header.Get("Authorization") != "Bearer access-token-two" {
			status = http.StatusUnauthorized
			body = `{"error":"invalid_token"}`
		} else {
			body = in.json(map[string]any{
				"sub": "user-123", "email": "ada@example.test", "preferred_username": "ada",
				"groups": []string{"engineering", "cluster-admins"}, "picture": "https://images.example.test/ada.png",
			})
		}
	default:
		status = http.StatusNotFound
		body = `{"error":"unexpected request"}`
	}
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    request,
	}, nil
}

func (in *protocolFixture) tokenResponse(request *http.Request) (int, string) {
	if request.Method != http.MethodPost {
		return http.StatusMethodNotAllowed, `{"error":"method_not_allowed"}`
	}
	if err := request.ParseForm(); err != nil {
		return http.StatusBadRequest, `{"error":"invalid_request"}`
	}
	if request.Form.Get("client_id") != "dashboard-test-client" || request.Form.Get("client_secret") != "dashboard-test-secret" {
		return http.StatusUnauthorized, `{"error":"invalid_client"}`
	}
	if in.tokenStatus != 0 {
		return in.tokenStatus, `{"error":"invalid_grant"}`
	}
	var (
		nonce       string
		accessToken = "access-token-one"
	)
	switch request.Form.Get("grant_type") {
	case "authorization_code":
		if request.Form.Get("code") != "authorization-code" || request.Form.Get("redirect_uri") != "https://dashboard.example.test/api/v1/oidc/callback" {
			return http.StatusBadRequest, `{"error":"invalid_grant"}`
		}
		verifier := request.Form.Get("code_verifier")
		verifierHash := sha256.Sum256([]byte(verifier))
		challenge := base64.RawURLEncoding.EncodeToString(verifierHash[:])
		in.mu.RLock()
		state := in.state
		in.mu.RUnlock()
		stateHash := sha256.Sum256([]byte(state.Verifier))
		if verifier != state.Verifier || challenge != base64.RawURLEncoding.EncodeToString(stateHash[:]) {
			return http.StatusBadRequest, `{"error":"invalid_grant"}`
		}
		nonce = state.Nonce
	case "refresh_token":
		if request.Form.Get("refresh_token") != "refresh-token-one" {
			return http.StatusBadRequest, `{"error":"invalid_grant"}`
		}
		nonce = "refresh-nonce"
		accessToken = "access-token-two"
	default:
		return http.StatusBadRequest, `{"error":"unsupported_grant_type"}`
	}
	if in.nonceOverride != "" {
		nonce = in.nonceOverride
	}
	idToken, err := in.signIDToken("user-123", nonce)
	if err != nil {
		return http.StatusInternalServerError, `{"error":"server_error"}`
	}
	response := map[string]any{
		"access_token": accessToken, "token_type": "Bearer", "expires_in": 3600,
		"scope": "openid profile email groups",
	}
	if !in.omitIDToken {
		response["id_token"] = idToken
	}
	if in.tokenType != "" {
		response["token_type"] = in.tokenType
	}
	if request.Form.Get("grant_type") == "authorization_code" {
		response["refresh_token"] = "refresh-token-one"
	}
	return http.StatusOK, in.json(response)
}

func (in *protocolFixture) signIDToken(subject, nonce string) (string, error) {
	return in.signIDTokenClaims(map[string]any{
		"iss": in.issuer, "sub": subject, "aud": "dashboard-test-client",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Add(-time.Second).Unix(), "nonce": nonce,
	})
}

func (in *protocolFixture) signIDTokenClaims(claims map[string]any) (string, error) {
	header, err := json.Marshal(map[string]string{"alg": "RS256", "kid": "dashboard-test-key", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(cryptorand.Reader, in.key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (in *protocolFixture) json(value any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func initializeProtocolFixture(t *testing.T, fixture *protocolFixture) *Manager {
	t.Helper()
	previousTransport := http.DefaultTransport
	http.DefaultTransport = oidcRoundTripFunc(fixture.roundTrip)
	t.Cleanup(func() { http.DefaultTransport = previousTransport })
	configureOIDCTestSessionKey(t)
	client.SetSessionCookieSecure(true)
	t.Cleanup(func() { client.SetSessionCookieSecure(true) })
	setOIDCFlag(t, "oidc-issuer-url", fixture.issuer)
	setOIDCFlag(t, "oidc-client-id", "dashboard-test-client")
	setOIDCFlag(t, "oidc-client-secret", "dashboard-test-secret")
	setOIDCFlag(t, "oidc-redirect-url", "https://dashboard.example.test/api/v1/oidc/callback")
	setOIDCFlag(t, "oidc-scopes", "profile email groups")
	setOIDCFlag(t, "oidc-groups-claim", "groups")
	setOIDCFlag(t, "oidc-picture-claim", "picture")
	setOIDCFlag(t, "session-cookie-secure", "true")
	setOIDCFlag(t, "anonymous-admin", "false")
	setManagerForTest(t, nil)
	if err := Initialize(); err != nil {
		t.Fatalf("Initialize() against signed provider: %v", err)
	}
	return current()
}

func TestOIDCLoginAndRefreshAgainstSignedProvider(t *testing.T) {
	for _, test := range []struct {
		name           string
		responseMode   string
		callbackMethod string
	}{
		{name: "query callback", responseMode: "query", callbackMethod: http.MethodGet},
		{name: "form post callback", responseMode: "form_post", callbackMethod: http.MethodPost},
	} {
		t.Run(test.name, func(t *testing.T) {
			responseModes := []string{"query"}
			if test.responseMode == "form_post" {
				responseModes = append(responseModes, "form_post")
			}
			fixture := newProtocolFixture(t, responseModes)
			initializeProtocolFixture(t, fixture)
			state, stateCookie := beginProtocolLogin(t, fixture, test.responseMode)
			session := completeProtocolLogin(t, state, stateCookie, test.callbackMethod)
			if !session.OIDC || session.Subject != "user-123" || session.AccessToken != "access-token-one" || session.RefreshToken != "refresh-token-one" {
				t.Errorf("OIDC session token data = %#v", session)
			}
			if session.Name != "user-123" || session.Email != "ada@example.test" || session.Picture != "https://images.example.test/ada.png" ||
				strings.Join(session.Groups, ",") != "engineering,cluster-admins" || strings.Join(session.Scopes, ",") != "openid,profile,email,groups" {
				t.Errorf("OIDC session profile = %#v", session)
			}

			session.TokenExpiry = time.Now().Add(-time.Second)
			refreshed, err := RefreshSession(t.Context(), session)
			if err != nil || !refreshed {
				t.Fatalf("RefreshSession() = %t, %v", refreshed, err)
			}
			if session.AccessToken != "access-token-two" || session.RefreshToken != "refresh-token-one" ||
				session.Subject != "user-123" || session.TokenExpiry.Before(time.Now()) {
				t.Errorf("refreshed session = %#v", session)
			}
		})
	}
}

func beginProtocolLogin(t *testing.T, fixture *protocolFixture, responseMode string) (authorizationState, *http.Cookie) {
	t.Helper()
	loginResponse := httptest.NewRecorder()
	BeginLogin(loginResponse, httptest.NewRequest(http.MethodPost, "https://dashboard.example.test/api/v1/oidc/login", nil))
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("BeginLogin() status = %d", loginResponse.Code)
	}
	var loginBody map[string]string
	if err := json.Unmarshal(loginResponse.Body.Bytes(), &loginBody); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	authorizationURL, err := url.Parse(loginBody["url"])
	if err != nil {
		t.Fatalf("parse authorization URL: %v", err)
	}
	wantResponseMode := ""
	if responseMode == "form_post" {
		wantResponseMode = "form_post"
	}
	if authorizationURL.Query().Get("response_mode") != wantResponseMode {
		t.Fatalf("authorization response mode = %q, want %q", authorizationURL.Query().Get("response_mode"), wantResponseMode)
	}
	var stateCookie *http.Cookie
	for _, cookie := range loginResponse.Result().Cookies() {
		if cookie.Name == stateCookieName {
			stateCookie = cookie
			break
		}
	}
	if stateCookie == nil {
		t.Fatal("BeginLogin() did not create a state cookie")
	}
	payload, err := client.OpenSessionData(statePurpose, stateCookie.Value)
	if err != nil {
		t.Fatalf("open state cookie: %v", err)
	}
	var state authorizationState
	if err := json.Unmarshal(payload, &state); err != nil {
		t.Fatalf("decode state cookie: %v", err)
	}
	fixture.mu.Lock()
	fixture.state = state
	fixture.mu.Unlock()
	return state, stateCookie
}

func completeProtocolLogin(t *testing.T, state authorizationState, stateCookie *http.Cookie, callbackMethod string) *client.BrowserSession {
	t.Helper()
	callbackURL := "https://dashboard.example.test/api/v1/oidc/callback"
	values := url.Values{"state": {state.State}, "code": {"authorization-code"}}
	var callbackRequest *http.Request
	if callbackMethod == http.MethodPost {
		callbackRequest = httptest.NewRequest(http.MethodPost, callbackURL, strings.NewReader(values.Encode()))
		callbackRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		callbackRequest = httptest.NewRequest(http.MethodGet, callbackURL+"?"+values.Encode(), nil)
	}
	callbackRequest.AddCookie(stateCookie)
	callbackResponse := httptest.NewRecorder()
	CompleteLogin(callbackResponse, callbackRequest)
	if callbackResponse.Code != http.StatusSeeOther || callbackResponse.Header().Get("Location") != "/" {
		t.Fatalf("CompleteLogin() = %d %q", callbackResponse.Code, callbackResponse.Header().Get("Location"))
	}
	sessionRequest := httptest.NewRequest(http.MethodGet, "https://dashboard.example.test/", nil)
	for _, cookie := range callbackResponse.Result().Cookies() {
		if strings.HasPrefix(cookie.Name, "kd-session-") && cookie.Value != "" {
			sessionRequest.AddCookie(cookie)
		}
	}
	session, err := client.ReadBrowserSession(sessionRequest)
	if err != nil {
		t.Fatalf("ReadBrowserSession() after OIDC callback: %v", err)
	}
	return session
}

func TestEnabledTracksOIDCManager(t *testing.T) {
	setManagerForTest(t, nil)
	if Enabled() {
		t.Fatal("Enabled() = true with no manager")
	}
	setManagerForTest(t, testManager("query"))
	if !Enabled() {
		t.Fatal("Enabled() = false with a configured manager")
	}
}

func TestExchangeCallbackRejectsInvalidTokenResponses(t *testing.T) {
	fixture := newProtocolFixture(t, []string{"query"})
	instance := initializeProtocolFixture(t, fixture)
	state := authorizationState{Nonce: "expected-nonce", Verifier: strings.Repeat("v", 43)}
	fixture.mu.Lock()
	fixture.state = state
	fixture.mu.Unlock()

	tests := []struct {
		name      string
		configure func()
		wantErr   string
	}{
		{name: "token endpoint error", configure: func() { fixture.tokenStatus = http.StatusBadRequest }},
		{name: "non bearer token", configure: func() { fixture.tokenType = "MAC" }, wantErr: "bearer access token"},
		{name: "missing ID token", configure: func() { fixture.omitIDToken = true }, wantErr: "ID token"},
		{name: "nonce mismatch", configure: func() { fixture.nonceOverride = "unexpected-nonce" }, wantErr: "nonce"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture.tokenStatus = 0
			fixture.tokenType = ""
			fixture.omitIDToken = false
			fixture.nonceOverride = ""
			test.configure()
			_, err := exchangeCallback(t.Context(), instance, url.Values{"code": {"authorization-code"}}, state)
			if err == nil {
				t.Fatal("exchangeCallback() unexpectedly accepted an invalid response")
			}
			if test.wantErr != "" && !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("exchangeCallback() error = %v, want containing %q", err, test.wantErr)
			}
		})
	}
}

func TestVerifyIDTokenValidatesAccessHashAndAuthorizedParty(t *testing.T) {
	fixture := newProtocolFixture(t, []string{"query"})
	instance := initializeProtocolFixture(t, fixture)
	accessToken := "signed-access-token"
	hash := sha256.Sum256([]byte(accessToken))
	baseClaims := func() map[string]any {
		return map[string]any{
			"iss": fixture.issuer, "sub": "user-123", "aud": []string{"dashboard-test-client", "another-client"},
			"azp": "dashboard-test-client", "exp": time.Now().Add(time.Hour).Unix(),
			"iat": time.Now().Add(-time.Second).Unix(),
		}
	}

	tests := []struct {
		name    string
		mutate  func(map[string]any)
		wantErr string
	}{
		{name: "valid access token hash", mutate: func(claims map[string]any) {
			claims["at_hash"] = base64.RawURLEncoding.EncodeToString(hash[:len(hash)/2])
		}},
		{name: "mismatched access token hash", mutate: func(claims map[string]any) { claims["at_hash"] = "wrong-hash" }, wantErr: "access-token hash"},
		{name: "malformed authorized party", mutate: func(claims map[string]any) { claims["azp"] = 42 }, wantErr: "authorized-party claim"},
		{name: "wrong authorized party", mutate: func(claims map[string]any) { claims["azp"] = "another-client" }, wantErr: "authorized-party claim does not match"},
		{name: "missing authorized party for multiple audiences", mutate: func(claims map[string]any) { delete(claims, "azp") }, wantErr: "authorized-party claim does not match"},
		{name: "future issued-at", mutate: func(claims map[string]any) { claims["iat"] = time.Now().Add(6 * time.Minute).Unix() }, wantErr: "issued-at claim"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			claims := baseClaims()
			test.mutate(claims)
			rawToken, err := fixture.signIDTokenClaims(claims)
			if err != nil {
				t.Fatal(err)
			}
			_, err = instance.verifyIDToken(t.Context(), rawToken, accessToken)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("verifyIDToken() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("verifyIDToken() error = %v, want containing %q", err, test.wantErr)
			}
		})
	}
}
