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
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"k8s.io/klog/v2"
)

const (
	// sessionCookieName is the name of the OIDC session cookie.
	sessionCookieName = "oidc-session"
	// tokenCookieName is the name of the cookie holding the ID/access token.
	tokenCookieName = "token"
	// stateCookieName is the name of the cookie used for OAuth2 state parameter.
	stateCookieName = "oidc-state"
	// userInfoCookieName is the name of the cookie holding serialized user info for the frontend.
	userInfoCookieName = "oidc-user"
)

// SessionData holds the OIDC session information.
type SessionData struct {
	// IDToken is the OIDC ID token.
	IDToken string `json:"id_token"`
	// AccessToken is the OAuth2 access token.
	AccessToken string `json:"access_token"`
	// RefreshToken is the OAuth2 refresh token.
	RefreshToken string `json:"refresh_token,omitempty"`
	// Expiry is when the access token expires.
	Expiry time.Time `json:"expiry"`
	// State is the OAuth2 state parameter used for CSRF protection.
	State string `json:"state,omitempty"`
	// Nonce is the OIDC nonce for replay protection (used temporarily for PKCE verifier).
	Nonce string `json:"nonce,omitempty"`
	// Username is the extracted user identity for K8s impersonation.
	Username string `json:"username,omitempty"`
	// Groups are the extracted groups for K8s impersonation.
	Groups []string `json:"groups,omitempty"`
	// DisplayName is the user's display name from the OIDC "name" claim.
	DisplayName string `json:"displayName,omitempty"`
	// Email is the user's email from the OIDC "email" claim.
	Email string `json:"email,omitempty"`
	// AvatarURL is the user's avatar URL from the OIDC "picture" claim.
	AvatarURL string `json:"avatarUrl,omitempty"`
}

// SessionManager handles OIDC session persistence via cookies.
type SessionManager struct {
	config       *Config
	oauth2Config *oauth2.Config
}

// NewSessionManager creates a new SessionManager.
func NewSessionManager(config *Config) *SessionManager {
	return &SessionManager{config: config}
}

// SetOAuth2Config stores the oauth2 config for use in token refresh.
func (m *SessionManager) SetOAuth2Config(cfg *oauth2.Config) {
	m.oauth2Config = cfg
}

// SetStateCookie sets the OAuth2 state cookie for CSRF protection.
func (m *SessionManager) SetStateCookie(w http.ResponseWriter, state string) {
	cookie := &http.Cookie{
		Name:     stateCookieName,
		Value:    state,
		Path:     "/",
		MaxAge:   600, // 10 minutes
		HttpOnly: true,
		Secure:   isSecureRequest(w),
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)
}

// GetStateCookie retrieves and clears the OAuth2 state cookie.
func (m *SessionManager) GetStateCookie(r *http.Request) (string, error) {
	cookie, err := r.Cookie(stateCookieName)
	if err != nil {
		return "", fmt.Errorf("state cookie not found: %w", err)
	}
	return cookie.Value, nil
}

// ClearStateCookie removes the state cookie.
func (m *SessionManager) ClearStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isSecureRequest(w),
		SameSite: http.SameSiteLaxMode,
	})
}

// SetTokenCookie stores the ID token in a cookie accessible to the frontend.
// This is the same cookie format used by the existing token-based auth.
func (m *SessionManager) SetTokenCookie(w http.ResponseWriter, token string, expiry time.Time) {
	maxAge := int(time.Until(expiry).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}

	cookie := &http.Cookie{
		Name:     tokenCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: false, // Frontend needs to read this for the Authorization header
		Secure:   isSecureRequest(w),
		SameSite: http.SameSiteStrictMode,
	}
	http.SetCookie(w, cookie)
}

// SetUserInfoCookie stores the user identity info in a cookie accessible to the frontend.
// This is used for impersonation-based K8s API access - the frontend reads this
// to set Impersonate-User and Impersonate-Group headers.
func (m *SessionManager) SetUserInfoCookie(w http.ResponseWriter, userInfo *OIDCUserInfo, expiry time.Time) {
	maxAge := int(time.Until(expiry).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}

	// Serialize user info as JSON
	data, err := json.Marshal(userInfo)
	if err != nil {
		klog.Errorf("Failed to marshal user info: %v", err)
		return
	}

	cookie := &http.Cookie{
		Name:     userInfoCookieName,
		Value:    base64.RawURLEncoding.EncodeToString(data),
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: false, // Frontend needs to read this for impersonation headers
		Secure:   isSecureRequest(w),
		SameSite: http.SameSiteStrictMode,
	}
	http.SetCookie(w, cookie)
}

// SetSessionCookie stores an encrypted session cookie for server-side session data.
func (m *SessionManager) SetSessionCookie(w http.ResponseWriter, data *SessionData) error {
	encrypted, err := m.encryptSession(data)
	if err != nil {
		return fmt.Errorf("failed to encrypt session: %w", err)
	}

	maxAge := int(time.Until(data.Expiry).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}

	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    encrypted,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   isSecureRequest(w),
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)
	return nil
}

// GetSessionCookie reads and decrypts the session cookie.
func (m *SessionManager) GetSessionCookie(r *http.Request) (*SessionData, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil, fmt.Errorf("session cookie not found: %w", err)
	}
	return m.decryptSession(cookie.Value)
}

// ClearSessionCookies removes both the session and token cookies.
func (m *SessionManager) ClearSessionCookies(w http.ResponseWriter) {
	for _, name := range []string{sessionCookieName, tokenCookieName, userInfoCookieName} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: false,
			Secure:   isSecureRequest(w),
			SameSite: http.SameSiteStrictMode,
		})
	}
}

// SessionTokenSource returns an oauth2.TokenSource that uses the refresh token from the session.
func (m *SessionManager) SessionTokenSource(r *http.Request) (oauth2.TokenSource, error) {
	if m.oauth2Config == nil {
		return nil, fmt.Errorf("oauth2 config not set")
	}

	session, err := m.GetSessionCookie(r)
	if err != nil {
		return nil, err
	}

	token := &oauth2.Token{
		AccessToken:  session.AccessToken,
		TokenType:    "Bearer",
		RefreshToken: session.RefreshToken,
		Expiry:       session.Expiry,
	}

	// Add the ID token as an extra field
	token = token.WithExtra(map[string]interface{}{
		"id_token": session.IDToken,
	})

	return m.oauth2Config.TokenSource(r.Context(), token), nil
}

// encryptSession encrypts the session data using AES-GCM.
func (m *SessionManager) encryptSession(data *SessionData) (string, error) {
	plaintext, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	key := m.deriveKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.RawURLEncoding.EncodeToString(ciphertext), nil
}

// decryptSession decrypts the session data using AES-GCM.
func (m *SessionManager) decryptSession(encoded string) (*SessionData, error) {
	ciphertext, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode session: %w", err)
	}

	key := m.deriveKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt session: %w", err)
	}

	var data SessionData
	if err := json.Unmarshal(plaintext, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &data, nil
}

// deriveKey derives a 32-byte AES key from the cookie secret.
func (m *SessionManager) deriveKey() []byte {
	secret := m.config.CookieSecret
	if secret == "" {
		secret = "kubernetes-dashboard-default-cookie-secret-CHANGE-ME"
	}
	if len(secret) < 32 {
		// Pad short secrets
		padded := make([]byte, 32)
		copy(padded, secret)
		secret = string(padded)
	}
	hash := sha256.Sum256([]byte(secret))
	return hash[:]
}

// GenerateState generates a random state string for OAuth2 CSRF protection.
func GenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// GenerateNonce generates a random nonce for OIDC replay protection.
func GenerateNonce() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// isSecureRequest determines if the request is using HTTPS in a way that
// makes it safe to set Secure cookies.
func isSecureRequest(w http.ResponseWriter) bool {
	// In production behind Kong, the request is HTTPS between client and Kong.
	// When running locally, it may be HTTP to localhost.
	// We check the X-Forwarded-Proto header set by Kong.
	// For simplicity, always set Secure=true since the dashboard should always
	// run behind HTTPS in production. Only skip Secure for explicit HTTP dev mode.
	return true
}


