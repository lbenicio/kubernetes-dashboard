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

		enabled := cfg.IsEnabled()
		response := &v1.OIDCConfig{
			Enabled:      enabled,
			ProviderName: cfg.ProviderName,
			ProviderURL:  cfg.IssuerURL,
			ClientID:     cfg.ClientID,
			Scopes:       cfg.DefaultScopes(),
		}

		writeJSON(w, http.StatusOK, response)
	}
}

// HandleLogin initiates the OIDC authorization code flow.
// It generates the authorization URL and returns it to the frontend.
func HandleLogin(provider *oidcpkg.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		redirectURL, verifier, state, err := provider.AuthCodeURL()
		if err != nil {
			klog.ErrorS(err, "Failed to generate OIDC auth URL")
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		// Store the PKCE verifier and state in an encrypted session cookie
		// for retrieval during the callback.
		sessionData := &oidcpkg.SessionData{
			State:  state,
			Nonce:  verifier,
			Expiry: time.Now().Add(10 * time.Minute),
		}
		if err := provider.Session().SetSessionCookie(w, sessionData); err != nil {
			klog.ErrorS(err, "Failed to set OIDC session cookie")
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create session"})
			return
		}

		// Also set the state in a plain cookie for CSRF check
		provider.Session().SetStateCookie(w, state)

		writeJSON(w, http.StatusOK, &v1.OIDCLoginResponse{
			RedirectURL: redirectURL,
		})
	}
}

// HandleCallback processes the OIDC callback from the identity provider.
func HandleCallback(provider *oidcpkg.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")

		if code == "" {
			errMsg := r.URL.Query().Get("error")
			errDesc := r.URL.Query().Get("error_description")
			klog.ErrorS(nil, "OIDC callback error", "error", errMsg, "description", errDesc)
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error":             errMsg,
				"error_description": errDesc,
			})
			return
		}

		// Validate state parameter against the cookie
		cookieState, err := provider.Session().GetStateCookie(r)
		if err != nil {
			klog.ErrorS(err, "State cookie validation failed")
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid state"})
			return
		}

		if state != cookieState {
			klog.ErrorS(nil, "OIDC state mismatch", "expected", cookieState, "got", state)
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "state mismatch"})
			return
		}

		// Retrieve the PKCE verifier from the session cookie
		sessionData, err := provider.Session().GetSessionCookie(r)
		if err != nil {
			klog.ErrorS(err, "Failed to get OIDC session cookie")
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "session expired"})
			return
		}

		verifier := sessionData.Nonce

		// Exchange the authorization code for tokens
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		token, err := provider.Exchange(ctx, code, verifier)
		if err != nil {
			klog.ErrorS(err, "OIDC token exchange failed")
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		// Create session and set cookies (includes JWKS validation + user extraction)
		userInfo, err := provider.CreateSession(w, r, token, state)
		if err != nil {
			klog.ErrorS(err, "Failed to create OIDC session")
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		// Set a cookie with user info for the frontend (non-HttpOnly so JS can read it)
		provider.Session().SetUserInfoCookie(w, userInfo, token.Expiry)

		// Redirect browser back to the dashboard root
		redirectPath := getRedirectPath(r)
		http.Redirect(w, r, redirectPath, http.StatusFound)
	}
}

// HandleSessionInfo returns the current session's user info for the frontend.
// This is called by the frontend to retrieve impersonation info after page load.
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

		userInfo := &v1.OIDCUserInfo{
			Username:    sessionData.Username,
			Groups:      sessionData.Groups,
			DisplayName: sessionData.DisplayName,
			Email:       sessionData.Email,
			AvatarURL:   sessionData.AvatarURL,
		}

		writeJSON(w, http.StatusOK, &v1.OIDCSession{
			Token: sessionData.IDToken,
			User:  *userInfo,
		})
	}
}

// HandleRefresh refreshes the OIDC tokens using the refresh token from the session.
func HandleRefresh(provider *oidcpkg.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenSource, err := provider.Session().SessionTokenSource(r)
		if err != nil {
			klog.ErrorS(err, "Failed to get token source")
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "no valid session"})
			return
		}

		newToken, err := tokenSource.Token()
		if err != nil {
			klog.ErrorS(err, "Token refresh failed")
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "token refresh failed"})
			return
		}

		idToken, err := provider.ExtractIDToken(newToken)
		if err != nil {
			idToken = newToken.AccessToken
		}

		// Re-validate and extract user info from new ID token
		userInfo, err := provider.ValidateAndExtractUser(r.Context(), idToken)
		if err != nil {
			klog.ErrorS(err, "Failed to validate refreshed ID token")
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "token validation failed"})
			return
		}

		// Update session cookies
		sessionData := &oidcpkg.SessionData{
			IDToken:      idToken,
			AccessToken:  newToken.AccessToken,
			RefreshToken: newToken.RefreshToken,
			Expiry:       newToken.Expiry,
			Username:     userInfo.Username,
			Groups:       userInfo.Groups,
			DisplayName:  userInfo.DisplayName,
			Email:        userInfo.Email,
			AvatarURL:    userInfo.AvatarURL,
		}

		if err := provider.Session().SetSessionCookie(w, sessionData); err != nil {
			klog.ErrorS(err, "Failed to update session cookie")
		}

		provider.Session().SetTokenCookie(w, idToken, newToken.Expiry)
		provider.Session().SetUserInfoCookie(w, userInfo, newToken.Expiry)

		writeJSON(w, http.StatusOK, &v1.OIDCSession{
			Token: idToken,
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

	redirectURL := url.URL{
		Scheme: proto,
		Host:   host,
		Path:   "/",
	}
	return redirectURL.String()
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	resp, err := json.Marshal(data)
	if err != nil {
		klog.ErrorS(err, "Failed to marshal JSON response")
		return
	}
	_, _ = w.Write(resp)
}
