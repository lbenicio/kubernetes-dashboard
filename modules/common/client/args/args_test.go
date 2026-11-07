// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

package args

import (
	"testing"
	"time"
)

func TestEnvBool(t *testing.T) {
	t.Setenv("DASHBOARD_TEST_BOOL", "")
	if got := envBool("DASHBOARD_TEST_BOOL", true); !got {
		t.Fatal("empty environment value did not use fallback")
	}
	t.Setenv("DASHBOARD_TEST_BOOL", "false")
	if got := envBool("DASHBOARD_TEST_BOOL", true); got {
		t.Fatal("false environment value was ignored")
	}
	t.Setenv("DASHBOARD_TEST_BOOL", "invalid")
	if got := envBool("DASHBOARD_TEST_BOOL", true); !got {
		t.Fatal("invalid environment value did not use fallback")
	}
}

func TestEnsureRequiresTokenExchangeForClusterContext(t *testing.T) {
	oldEnabled, oldEndpoint := *argClusterContextEnabled, *argTokenExchangeEndpoint
	t.Cleanup(func() {
		*argClusterContextEnabled = oldEnabled
		*argTokenExchangeEndpoint = oldEndpoint
	})

	*argClusterContextEnabled = true
	*argTokenExchangeEndpoint = ""
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("Ensure() did not reject missing token exchange endpoint")
			}
		}()
		Ensure()
	}()

	*argTokenExchangeEndpoint = "https://auth.example.test/exchange"
	Ensure()
}

func TestClientArgumentAccessors(t *testing.T) {
	old := struct {
		sessionKey, tokenExchange             string
		anonymousAdmin, cache, clusterContext bool
		cacheSize                             int
		cacheTTL, debounce                    time.Duration
	}{
		*argSessionKey, *argTokenExchangeEndpoint, *argAnonymousAdmin, *argCacheEnabled,
		*argClusterContextEnabled, *argCacheSize, *argCacheTTL, *argCacheRefreshDebounce,
	}
	t.Cleanup(func() {
		*argSessionKey, *argTokenExchangeEndpoint = old.sessionKey, old.tokenExchange
		*argAnonymousAdmin, *argCacheEnabled = old.anonymousAdmin, old.cache
		*argClusterContextEnabled, *argCacheSize = old.clusterContext, old.cacheSize
		*argCacheTTL, *argCacheRefreshDebounce = old.cacheTTL, old.debounce
	})

	*argSessionKey, *argTokenExchangeEndpoint = "encoded-key", "https://auth.example.test/exchange"
	*argAnonymousAdmin, *argCacheEnabled = true, false
	*argClusterContextEnabled, *argCacheSize = true, 42
	*argCacheTTL, *argCacheRefreshDebounce = 2*time.Minute, 3*time.Second

	if SessionKey() != "encoded-key" || TokenExchangeEndpoint() != "https://auth.example.test/exchange" {
		t.Fatal("session or token exchange accessor returned an unexpected value")
	}
	if !AnonymousAdminEnabled() || CacheEnabled() || !ClusterContextEnabled() || CacheSize() != 42 {
		t.Fatal("boolean or cache size accessors returned unexpected values")
	}
	if CacheTTL() != 2*time.Minute || CacheRefreshDebounce() != 3*time.Second {
		t.Fatal("cache duration accessors returned unexpected values")
	}
}
