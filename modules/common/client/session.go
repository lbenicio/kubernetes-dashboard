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

package client

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	browserSessionCookiePrefix = "kd-session-"
	maxSessionCookieChunks     = 8
	sessionCookieChunkSize     = 3000
	sessionCookiePurpose       = "kubernetes-dashboard-browser-session-v1"
)

var (
	sessionKeyMu sync.RWMutex
	sessionKey   []byte
	cookieSecure = true
)

// BrowserSession is encrypted before it is sent to the browser. Its Kubernetes
// token is never exposed to JavaScript.
type BrowserSession struct {
	KubernetesToken string    `json:"kubernetesToken"`
	OIDC            bool      `json:"oidc,omitempty"`
	Name            string    `json:"name,omitempty"`
	Email           string    `json:"email,omitempty"`
	Picture         string    `json:"picture,omitempty"`
	Groups          []string  `json:"groups,omitempty"`
	Scopes          []string  `json:"scopes,omitempty"`
	Subject         string    `json:"subject,omitempty"`
	AccessToken     string    `json:"accessToken,omitempty"`
	RefreshToken    string    `json:"refreshToken,omitempty"`
	TokenExpiry     time.Time `json:"tokenExpiry,omitempty"`
	ExpiresAt       time.Time `json:"expiresAt"`
}

// ConfigureSessionKey configures the shared 256-bit key used to protect browser
// sessions between the Auth and API services. The key must be base64 encoded.
func ConfigureSessionKey(encoded string) error {
	key, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		key, err = base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	}
	if err != nil || len(key) != 32 {
		return errors.New("session key must be a base64-encoded 32-byte value")
	}

	sessionKeyMu.Lock()
	sessionKey = append(sessionKey[:0], key...)
	sessionKeyMu.Unlock()
	return nil
}

func HasSessionKey() bool {
	sessionKeyMu.RLock()
	defer sessionKeyMu.RUnlock()
	return len(sessionKey) == 32
}

// SetSessionCookieSecure controls whether session cookies include the Secure
// attribute. It defaults to true; only local HTTP development should disable it.
func SetSessionCookieSecure(secure bool) {
	cookieSecure = secure
}

func SetBrowserSessionCookie(w http.ResponseWriter, r *http.Request, session BrowserSession) error {
	if session.KubernetesToken == "" || session.ExpiresAt.IsZero() || !session.ExpiresAt.After(time.Now()) {
		return errors.New("invalid browser session")
	}
	payload, err := json.Marshal(session)
	if err != nil {
		return err
	}
	sealed, err := sealSessionPayload(sessionCookiePurpose, payload)
	if err != nil {
		return err
	}
	encoded := base64.RawURLEncoding.EncodeToString(sealed)
	chunks := (len(encoded) + sessionCookieChunkSize - 1) / sessionCookieChunkSize
	if chunks == 0 || chunks > maxSessionCookieChunks {
		return errors.New("browser session is too large to store safely")
	}
	maxAge := int(time.Until(session.ExpiresAt).Seconds())
	for i := 0; i < chunks; i++ {
		start := i * sessionCookieChunkSize
		end := min(start+sessionCookieChunkSize, len(encoded))
		writeSessionCookie(w, r, browserSessionCookiePrefix+strconv.Itoa(i), encoded[start:end], maxAge)
	}
	for i := chunks; i < maxSessionCookieChunks; i++ {
		writeSessionCookie(w, r, browserSessionCookiePrefix+strconv.Itoa(i), "", -1)
	}
	return nil
}

func ReadBrowserSession(r *http.Request) (*BrowserSession, error) {
	var encoded strings.Builder
	seen := false
	missing := false
	for i := 0; i < maxSessionCookieChunks; i++ {
		cookie, err := r.Cookie(browserSessionCookiePrefix + strconv.Itoa(i))
		if err != nil {
			missing = true
			continue
		}
		if missing {
			return nil, errors.New("invalid browser session cookie sequence")
		}
		seen = true
		encoded.WriteString(cookie.Value)
	}
	if !seen {
		return nil, http.ErrNoCookie
	}
	if encoded.Len() > sessionCookieChunkSize*maxSessionCookieChunks {
		return nil, errors.New("browser session is too large")
	}
	sealed, err := base64.RawURLEncoding.DecodeString(encoded.String())
	if err != nil {
		return nil, errors.New("invalid browser session")
	}
	payload, err := openSessionPayload(sessionCookiePurpose, sealed)
	if err != nil {
		return nil, err
	}
	var session BrowserSession
	if err := json.Unmarshal(payload, &session); err != nil {
		return nil, errors.New("invalid browser session")
	}
	if session.KubernetesToken == "" || session.ExpiresAt.IsZero() || !session.ExpiresAt.After(time.Now()) {
		return nil, errors.New("browser session has expired")
	}
	return &session, nil
}

func ClearBrowserSessionCookie(w http.ResponseWriter, r *http.Request) {
	for i := 0; i < maxSessionCookieChunks; i++ {
		writeSessionCookie(w, r, browserSessionCookiePrefix+strconv.Itoa(i), "", -1)
	}
}

// SealSessionData and OpenSessionData protect short-lived OIDC state using the
// same key as browser sessions, with purpose separation through AEAD.
func SealSessionData(purpose string, data []byte) (string, error) {
	sealed, err := sealSessionPayload(purpose, data)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func OpenSessionData(purpose, encoded string) ([]byte, error) {
	sealed, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("invalid protected data")
	}
	return openSessionPayload(purpose, sealed)
}

func WriteProtectedCookie(w http.ResponseWriter, r *http.Request, name, value string, maxAge int) {
	writeSessionCookie(w, r, name, value, maxAge)
}

func WriteProtectedCookieWithSameSite(w http.ResponseWriter, r *http.Request, name, value string, maxAge int, sameSite http.SameSite) {
	writeSessionCookieWithSameSite(w, r, name, value, maxAge, sameSite)
}

func ClearProtectedCookie(w http.ResponseWriter, r *http.Request, name string) {
	writeSessionCookie(w, r, name, "", -1)
}

func sealSessionPayload(purpose string, payload []byte) ([]byte, error) {
	block, err := sessionAEAD()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, block.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return append(nonce, block.Seal(nil, nonce, payload, []byte(purpose))...), nil
}

func openSessionPayload(purpose string, sealed []byte) ([]byte, error) {
	block, err := sessionAEAD()
	if err != nil {
		return nil, err
	}
	if len(sealed) < block.NonceSize()+block.Overhead() {
		return nil, errors.New("invalid protected data")
	}
	nonce := sealed[:block.NonceSize()]
	plain, err := block.Open(nil, nonce, sealed[block.NonceSize():], []byte(purpose))
	if err != nil {
		return nil, errors.New("invalid protected data")
	}
	return plain, nil
}

func sessionAEAD() (cipher.AEAD, error) {
	sessionKeyMu.RLock()
	key := append([]byte(nil), sessionKey...)
	sessionKeyMu.RUnlock()
	if len(key) != 32 {
		return nil, errors.New("browser session key is not configured")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func writeSessionCookie(w http.ResponseWriter, r *http.Request, name, value string, maxAge int) {
	writeSessionCookieWithSameSite(w, r, name, value, maxAge, http.SameSiteLaxMode)
}

func writeSessionCookieWithSameSite(w http.ResponseWriter, r *http.Request, name, value string, maxAge int, sameSite http.SameSite) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		Expires:  time.Now().Add(time.Duration(maxAge) * time.Second),
		HttpOnly: true,
		Secure:   cookieSecure,
		SameSite: sameSite,
	})
}
