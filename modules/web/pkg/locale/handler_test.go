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

package locale

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/pflag"
	"golang.org/x/text/language"
)

func TestGetAcceptLanguagePreferenceOrder(t *testing.T) {
	cases := []struct {
		name        string
		cookie      string
		environment string
		header      string
		want        string
	}{
		{name: "cookie", cookie: "fr", environment: "de", header: "ja", want: "fr"},
		{name: "environment", environment: "de", header: "ja", want: "de"},
		{name: "header", header: "ja", want: "ja"},
		{name: "no preference"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ACCEPT_LANGUAGE", tc.environment)
			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			context.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			context.Request.Header.Set("Accept-Language", tc.header)
			if tc.cookie != "" {
				context.Request.AddCookie(&http.Cookie{Name: "lang", Value: tc.cookie})
			}

			if got := getAcceptLanguage(context); got != tc.want {
				t.Errorf("getAcceptLanguage() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGetSupportedLocales(t *testing.T) {
	path := filepath.Join(t.TempDir(), "locales.json")
	if err := os.WriteFile(path, []byte(`{"translations":["de","zh-Hans"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	flag := pflag.Lookup("locale-config")
	old := flag.Value.String()
	t.Cleanup(func() { _ = flag.Value.Set(old) })
	if err := flag.Value.Set(path); err != nil {
		t.Fatal(err)
	}

	if got, want := getSupportedLocales(), []language.Tag{language.MustParse("de"), language.MustParse("zh-Hans")}; !equalTags(got, want) {
		t.Fatalf("getSupportedLocales() = %v, want %v", got, want)
	}

	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := getSupportedLocales(); len(got) != 0 {
		t.Fatalf("malformed locale config produced tags %v", got)
	}
}

func TestDetermineLocalizedDirSupportsLegacyChineseTags(t *testing.T) {
	public := filepath.Dir(expectedDefaultLocaleDir())
	for _, locale := range []string{"zh-Hans", "zh-Hant", "zh-Hant-HK"} {
		if err := os.MkdirAll(filepath.Join(public, locale), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	previous := supportedLocales
	supportedLocales = []language.Tag{
		language.MustParse("zh-Hans"),
		language.MustParse("zh-Hant"),
		language.MustParse("zh-Hant-HK"),
	}
	t.Cleanup(func() {
		supportedLocales = previous
		_ = os.RemoveAll(public)
	})

	cases := map[string]string{
		"zh-CN": filepath.Join(public, "zh-Hans"),
		"zh-TW": filepath.Join(public, "zh-Hant"),
		"zh-HK": filepath.Join(public, "zh-Hant-HK"),
	}
	for input, want := range cases {
		if got := determineLocalizedDir(input); got != want {
			t.Errorf("determineLocalizedDir(%q) = %q, want %q", input, got, want)
		}
	}
	if got, want := determineLocalizedDir("unsupported"), filepath.Join(public, defaultLocaleDir); got != want {
		t.Errorf("unsupported locale fallback = %q, want %q", got, want)
	}
}

func TestLocaleHandlerDisablesDirectoryListingAndDisablesHTMLCache(t *testing.T) {
	listingResponse := httptest.NewRecorder()
	listingContext, _ := gin.CreateTestContext(listingResponse)
	listingContext.Request = httptest.NewRequest(http.MethodGet, "/assets/", nil)
	localeHandler("/")(listingContext)
	if listingResponse.Code != http.StatusNotFound {
		t.Fatalf("directory listing status = %d, want %d", listingResponse.Code, http.StatusNotFound)
	}

	indexResponse := httptest.NewRecorder()
	indexContext, _ := gin.CreateTestContext(indexResponse)
	indexContext.Request = httptest.NewRequest(http.MethodGet, "/index.html", nil)
	localeHandler("")(indexContext)
	if indexResponse.Code == 0 {
		t.Error("index request did not produce a response")
	}
}

func equalTags(left, right []language.Tag) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func TestLocaleDirFallsBackForUnsupportedLanguage(t *testing.T) {
	if got, want := getLocaleDir("unsupported-language"), expectedDefaultLocaleDir(); got != want {
		t.Errorf("getLocaleDir() = %q, want fallback %q", got, want)
	}
}

func expectedDefaultLocaleDir() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(resolved), assetsDir, defaultLocaleDir)
}
