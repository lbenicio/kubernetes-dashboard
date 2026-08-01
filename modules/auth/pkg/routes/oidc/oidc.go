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
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	v1 "k8s.io/dashboard/auth/api/v1"
	oidcpkg "k8s.io/dashboard/auth/pkg/oidc"
	"k8s.io/klog/v2"
)

// HandleGetConfig returns the OIDC configuration to the frontend.
func HandleGetConfig(provider *oidcpkg.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := provider.GetConfig()
		response := &v1.OIDCConfig{
			Enabled:      cfg.IsEnabled(),
			ProviderName: cfg.ProviderName,
			ProviderURL:  cfg.IssuerURL,
			ClientID:     cfg.ClientID,
			Scopes:       cfg.DefaultScopes(),
		}
		writeJSON(w, http.StatusOK, response)
	}
}

// HandleLogin initiates the OIDC authorization code flow.
func HandleLogin(provider *oidcpkg.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		redirectURL, verifier, state, nonce, err := provider.AuthCodeURL()
		if err != nil {
			klog.ErrorS(err, "Failed to generate OIDC auth URL")
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		// Store PKCE verifier in both:
		// 1. Encrypted session cookie (backward compat for existing callback)
		// 2. In-memory store (for the new SPA-based callback, ITP-safe)
		sessionData := &oidcpkg.SessionData{
			State:  state,
			Nonce:  fmt.Sprintf("%s|%s", verifier, nonce),
			Expiry: time.Now().Add(10 * time.Minute),
		}
		if err := provider.Session().SetSessionCookie(w, sessionData); err != nil {
			klog.ErrorS(err, "Failed to set OIDC session cookie")
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create session"})
			return
		}

		provider.Session().SetStateCookie(w, state)

		// Store in in-memory PKCE store for the ITP-safe SPA callback flow.
		// The SPA will pass the state back to the exchange endpoint, which
		// looks up the verifier from this store instead of from cookies.
		provider.PKCEStore().Put(state, verifier, nonce)

		writeJSON(w, http.StatusOK, &v1.OIDCLoginResponse{
			RedirectURL: redirectURL,
			State:       state,
		})
	}
}

// HandleCallback processes the OIDC callback from the identity provider.
func HandleCallback(provider *oidcpkg.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")

		if code == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error":             r.URL.Query().Get("error"),
				"error_description": r.URL.Query().Get("error_description"),
			})
			return
		}

		// Validate state parameter against the cookie set during login.
		// This provides basic CSRF protection. The cookie was set in a
		// same-site XHR response, so ITP should not block it.
		cookieState, err := provider.Session().GetStateCookie(r)
		if err != nil || state != cookieState {
			klog.ErrorS(err, "State cookie validation failed")
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid state"})
			return
		}

		// Redirect to the SPA callback page. The Angular app uses hash routing
		// (/#/oidc-callback), so we must include the hash.
		dest := fmt.Sprintf("/#/oidc-callback?code=%s&state=%s",
			url.QueryEscape(code), url.QueryEscape(state))
		http.Redirect(w, r, dest, http.StatusFound)
	}
}

// HandleSessionInfo returns the current session's user info.
func HandleSessionInfo(provider *oidcpkg.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionData, err := provider.Session().GetSessionCookie(r)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"authenticated": "false",
				"error":         "no valid session",
			})
			return
		}

		writeJSON(w, http.StatusOK, &v1.OIDCSession{
			Token: sessionData.IDToken,
			User: v1.OIDCUserInfo{
				Username:    sessionData.Username,
				Groups:      sessionData.Groups,
				DisplayName: sessionData.DisplayName,
				Email:       sessionData.Email,
				AvatarURL:   sessionData.AvatarURL,
			},
		})
	}
}

// HandleRefresh refreshes the OIDC tokens.
func HandleRefresh(provider *oidcpkg.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenSource, err := provider.Session().SessionTokenSource(r)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "no valid session"})
			return
		}

		newToken, err := tokenSource.Token()
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "token refresh failed"})
			return
		}

		rawIDToken, err := provider.ExtractIDToken(newToken)
		if err != nil {
			rawIDToken = newToken.AccessToken
		}

		// Re-validate without nonce (already authenticated session)
		userInfo, err := provider.ValidateAndExtractUser(r.Context(), rawIDToken, "")
		if err != nil {
			klog.ErrorS(err, "Failed to validate refreshed ID token")
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "token validation failed"})
			return
		}

		sessionData := &oidcpkg.SessionData{
			IDToken:      rawIDToken,
			AccessToken:  newToken.AccessToken,
			RefreshToken: newToken.RefreshToken,
			Expiry:       newToken.Expiry,
			Username:     userInfo.Username,
			Groups:       userInfo.Groups,
			DisplayName:  userInfo.DisplayName,
			Email:        userInfo.Email,
			AvatarURL:    userInfo.AvatarURL,
		}

		provider.Session().SetSessionCookie(w, sessionData)
		provider.Session().SetTokenCookie(w, rawIDToken, newToken.Expiry)
		provider.Session().SetUserInfoCookie(w, userInfo, newToken.Expiry)
		provider.Session().SetCSRFCookie(w)

		writeJSON(w, http.StatusOK, &v1.OIDCSession{
			Token: rawIDToken,
			User: v1.OIDCUserInfo{
				Username:    userInfo.Username,
				Groups:      userInfo.Groups,
				DisplayName: userInfo.DisplayName,
				Email:       userInfo.Email,
				AvatarURL:   userInfo.AvatarURL,
			},
		})
	}
}

// HandleLogout clears the OIDC session.
func HandleLogout(provider *oidcpkg.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		provider.Session().ClearSessionCookies(w)
		writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
	}
}

// HandleExchange processes the OIDC authorization code from the SPA callback.
// This is the ITP-safe alternative to HandleCallback. Instead of receiving the
// code via a cross-site redirect (where Safari ITP blocks cookies), the SPA
// receives the code via the URL and passes it to this endpoint via same-site XHR.
// The PKCE verifier is looked up from the in-memory store, not from cookies.
func HandleExchange(provider *oidcpkg.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req := new(v1.OIDCCallbackRequest)
		if err := json.NewDecoder(r.Body).Decode(req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		if req.Code == "" || req.State == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "code and state are required"})
			return
		}

		// Look up PKCE verifier from in-memory store (ITP-safe, no cookies needed).
		verifier, nonce, ok := provider.PKCEStore().Get(req.State)
		if !ok {
			klog.Errorf("State not found in PKCE store: %s", req.State)
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid or expired state"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		token, err := provider.Exchange(ctx, req.Code, verifier)
		if err != nil {
			klog.ErrorS(err, "OIDC token exchange failed")
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		// Validate and create session.
		// Cookies set here are in response to a same-site XHR request
		// (the SPA is on k8sdb.lan), NOT a cross-site redirect.
		// Safari ITP does NOT block cookies in same-site XHR responses.
		userInfo, err := provider.CreateSession(w, r, token, nonce, req.State)
		if err != nil {
			klog.ErrorS(err, "Failed to create OIDC session")
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		provider.Session().SetUserInfoCookie(w, userInfo, token.Expiry)
		provider.Session().SetCSRFCookie(w)

		// Return user info in the JSON response so the SPA can store it
		// in sessionStorage without relying on cookies.
		writeJSON(w, http.StatusOK, &v1.OIDCSession{
			Token: "",
			User: v1.OIDCUserInfo{
				Username:    userInfo.Username,
				Groups:      userInfo.Groups,
				DisplayName: userInfo.DisplayName,
				Email:       userInfo.Email,
				AvatarURL:   userInfo.AvatarURL,
			},
		})
	}
}

// getRedirectPath determines the redirect path after OIDC callback.
func getRedirectPath(r *http.Request) string {
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		proto = "https"
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return (&url.URL{Scheme: proto, Host: host, Path: "/"}).String()
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		klog.ErrorS(err, "Failed to write JSON response")
	}
}
