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
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func configureTestSessionKey(t *testing.T) {
	t.Helper()
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
	if err := ConfigureSessionKey(key); err != nil {
		t.Fatalf("ConfigureSessionKey() error = %v", err)
	}
}

func TestConfigureSessionKey(t *testing.T) {
	if err := ConfigureSessionKey("invalid"); err == nil {
		t.Fatal("ConfigureSessionKey() accepted an invalid key")
	}
	if err := ConfigureSessionKey(base64.StdEncoding.EncodeToString([]byte("short"))); err == nil {
		t.Fatal("ConfigureSessionKey() accepted a key with the wrong length")
	}

	configureTestSessionKey(t)
	if !HasSessionKey() {
		t.Fatal("HasSessionKey() = false after configuring a valid key")
	}

	urlEncoded := base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("u", 32)))
	if err := ConfigureSessionKey(urlEncoded); err != nil {
		t.Fatalf("ConfigureSessionKey() rejected raw URL-safe base64: %v", err)
	}
}

func TestSetAndReadBrowserSessionCookie(t *testing.T) {
	configureTestSessionKey(t)
	previousSecure := cookieSecure
	SetSessionCookieSecure(true)
	t.Cleanup(func() { SetSessionCookieSecure(previousSecure) })

	session := BrowserSession{
		KubernetesToken: "kubernetes-id-token",
		OIDC:            true,
		Name:            "Ada Lovelace",
		Email:           "ada@example.test",
		Picture:         "https://images.example.test/ada.png",
		Groups:          []string{"engineering", "operators"},
		Scopes:          []string{"openid", "profile"},
		AccessToken:     "access-token",
		RefreshToken:    strings.Repeat("r", 4000),
		ExpiresAt:       time.Now().Add(time.Hour),
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "https://dashboard.example.test/", nil)
	if err := SetBrowserSessionCookie(recorder, request, session); err != nil {
		t.Fatalf("SetBrowserSessionCookie() error = %v", err)
	}

	cookies := recorder.Result().Cookies()
	if len(cookies) < 2 {
		t.Fatalf("session cookie was not split as expected: got %d cookies", len(cookies))
	}
	for _, cookie := range cookies {
		if cookie.MaxAge < 0 || cookie.Value == "" {
			continue
		}
		if !cookie.HttpOnly || !cookie.Secure || cookie.Path != "/" || cookie.SameSite != http.SameSiteLaxMode {
			t.Errorf("session cookie security attributes are incomplete: %#v", cookie)
		}
		request.AddCookie(cookie)
	}

	got, err := ReadBrowserSession(request)
	if err != nil {
		t.Fatalf("ReadBrowserSession() error = %v", err)
	}
	if got.KubernetesToken != session.KubernetesToken || got.RefreshToken != session.RefreshToken || got.Subject != session.Subject {
		t.Fatalf("ReadBrowserSession() returned a different session: %#v", got)
	}
	if got.Name != session.Name || got.Email != session.Email || got.Picture != session.Picture {
		t.Fatalf("ReadBrowserSession() lost profile claims: %#v", got)
	}
	if strings.Join(got.Groups, ",") != strings.Join(session.Groups, ",") || strings.Join(got.Scopes, ",") != strings.Join(session.Scopes, ",") {
		t.Fatalf("ReadBrowserSession() lost groups or scopes: %#v", got)
	}
}

func TestSetBrowserSessionCookieRejectsInvalidOrOversizedSessions(t *testing.T) {
	configureTestSessionKey(t)
	request := httptest.NewRequest(http.MethodGet, "https://dashboard.example.test/", nil)
	for _, session := range []BrowserSession{
		{ExpiresAt: time.Now().Add(time.Hour)},
		{KubernetesToken: "expired", ExpiresAt: time.Now().Add(-time.Minute)},
		{KubernetesToken: "missing expiry"},
	} {
		if err := SetBrowserSessionCookie(httptest.NewRecorder(), request, session); err == nil {
			t.Errorf("SetBrowserSessionCookie() accepted invalid session %#v", session)
		}
	}

	oversized := BrowserSession{
		KubernetesToken: "token",
		RefreshToken:    strings.Repeat("x", sessionCookieChunkSize*maxSessionCookieChunks),
		ExpiresAt:       time.Now().Add(time.Hour),
	}
	if err := SetBrowserSessionCookie(httptest.NewRecorder(), request, oversized); err == nil {
		t.Fatal("SetBrowserSessionCookie() accepted a session larger than the cookie limit")
	}
}

func TestReadBrowserSessionRejectsMissingMalformedAndExpiredCookies(t *testing.T) {
	configureTestSessionKey(t)
	t.Run("missing", func(t *testing.T) {
		if _, err := ReadBrowserSession(httptest.NewRequest(http.MethodGet, "https://dashboard.example.test/", nil)); !errors.Is(err, http.ErrNoCookie) {
			t.Fatalf("ReadBrowserSession() error = %v, want http.ErrNoCookie", err)
		}
	})

	t.Run("invalid sequence", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "https://dashboard.example.test/", nil)
		request.AddCookie(&http.Cookie{Name: browserSessionCookiePrefix + "1", Value: "part"})
		if _, err := ReadBrowserSession(request); err == nil {
			t.Fatal("ReadBrowserSession() accepted a cookie sequence without its first chunk")
		}
	})

	t.Run("malformed encoding", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "https://dashboard.example.test/", nil)
		request.AddCookie(&http.Cookie{Name: browserSessionCookiePrefix + "0", Value: "not-base64"})
		if _, err := ReadBrowserSession(request); err == nil {
			t.Fatal("ReadBrowserSession() accepted malformed cookie data")
		}
	})

	t.Run("expired payload", func(t *testing.T) {
		payload := []byte(`{"kubernetesToken":"token","expiresAt":"2000-01-01T00:00:00Z"}`)
		sealed, err := sealSessionPayload(sessionCookiePurpose, payload)
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodGet, "https://dashboard.example.test/", nil)
		request.AddCookie(&http.Cookie{Name: browserSessionCookiePrefix + "0", Value: base64.RawURLEncoding.EncodeToString(sealed)})
		if _, err := ReadBrowserSession(request); err == nil {
			t.Fatal("ReadBrowserSession() accepted an expired session")
		}
	})
}

func TestProtectedSessionDataUsesPurposeAndRejectsTampering(t *testing.T) {
	configureTestSessionKey(t)
	encoded, err := SealSessionData("oidc-state", []byte("state-value"))
	if err != nil {
		t.Fatalf("SealSessionData() error = %v", err)
	}
	got, err := OpenSessionData("oidc-state", encoded)
	if err != nil || string(got) != "state-value" {
		t.Fatalf("OpenSessionData() = %q, %v", got, err)
	}
	if _, err := OpenSessionData("browser-session", encoded); err == nil {
		t.Fatal("OpenSessionData() accepted data sealed for another purpose")
	}
	if _, err := OpenSessionData("oidc-state", "bad encoding"); err == nil {
		t.Fatal("OpenSessionData() accepted malformed base64")
	}
	if _, err := OpenSessionData("oidc-state", base64.RawURLEncoding.EncodeToString([]byte("short"))); err == nil {
		t.Fatal("OpenSessionData() accepted a payload shorter than the AEAD nonce")
	}
}

func TestClearBrowserAndProtectedCookies(t *testing.T) {
	previousSecure := cookieSecure
	SetSessionCookieSecure(false)
	t.Cleanup(func() { SetSessionCookieSecure(previousSecure) })
	request := httptest.NewRequest(http.MethodGet, "http://localhost/", nil)

	recorder := httptest.NewRecorder()
	ClearBrowserSessionCookie(recorder, request)
	cookies := recorder.Result().Cookies()
	if len(cookies) != maxSessionCookieChunks {
		t.Fatalf("ClearBrowserSessionCookie() wrote %d cookies, want %d", len(cookies), maxSessionCookieChunks)
	}
	for _, cookie := range cookies {
		if cookie.MaxAge >= 0 || cookie.Secure || !cookie.HttpOnly || cookie.Path != "/" {
			t.Errorf("cleared cookie has incorrect attributes: %#v", cookie)
		}
	}

	recorder = httptest.NewRecorder()
	WriteProtectedCookieWithSameSite(recorder, request, "state", "protected", 60, http.SameSiteNoneMode)
	stateCookie := recorder.Result().Cookies()[0]
	if stateCookie.SameSite != http.SameSiteNoneMode || stateCookie.Secure || !stateCookie.HttpOnly {
		t.Errorf("protected cookie has incorrect SameSite or security attributes: %#v", stateCookie)
	}

	recorder = httptest.NewRecorder()
	ClearProtectedCookie(recorder, request, "state")
	if cookie := recorder.Result().Cookies()[0]; cookie.MaxAge >= 0 {
		t.Errorf("ClearProtectedCookie() did not expire the cookie: %#v", cookie)
	}
}
