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

package args

import (
	"os"
	"strconv"
	"time"

	"github.com/spf13/pflag"
)

var (
	argSessionKey            = pflag.String("session-key", os.Getenv("DASHBOARD_SESSION_KEY"), "base64-encoded 32-byte key shared by the auth and API services for encrypted browser sessions")
	argAnonymousAdmin        = pflag.Bool("anonymous-admin", envBool("DASHBOARD_ANONYMOUS_ADMIN", false), "serve requests without login using the Dashboard service credentials; every reachable user receives those permissions")
	argCacheEnabled          = pflag.Bool("cache-enabled", true, "whether client cache should be enabled or not")
	argClusterContextEnabled = pflag.Bool("cluster-context-enabled", false, "whether multi-cluster cache context support should be enabled or not")
	argTokenExchangeEndpoint = pflag.String("token-exchange-endpoint", "", "endpoint used in multi-cluster cache to exchange tokens for context identifiers")
	argCacheSize             = pflag.Int("cache-size", 1000, "max number of cache entries")
	argCacheTTL              = pflag.Duration("cache-ttl", 10*time.Minute, "cache entry TTL")
	argCacheRefreshDebounce  = pflag.Duration("cache-refresh-debounce", 5*time.Second, "minimal time between cache refreshes in the background")
)

func envBool(name string, fallback bool) bool {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func Ensure() {
	if *argClusterContextEnabled && len(*argTokenExchangeEndpoint) == 0 {
		panic("token-exchange-endpoint must be set when cluster-context-enabled is set to true")
	}
}

func SessionKey() string {
	return *argSessionKey
}

func AnonymousAdminEnabled() bool {
	return *argAnonymousAdmin
}

func CacheEnabled() bool {
	return *argCacheEnabled
}

func ClusterContextEnabled() bool {
	return *argClusterContextEnabled
}

func TokenExchangeEndpoint() string {
	return *argTokenExchangeEndpoint
}

func CacheSize() int {
	return *argCacheSize
}

func CacheTTL() time.Duration {
	return *argCacheTTL
}

func CacheRefreshDebounce() time.Duration {
	return *argCacheRefreshDebounce
}
