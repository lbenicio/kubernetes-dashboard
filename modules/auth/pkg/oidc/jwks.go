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
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/klog/v2"
)

// extractUserInfoFromClaims extracts the user identity from OIDC claims.
// Uses configurable claim names from the provider configuration.
func extractUserInfoFromClaims(claims map[string]interface{}, config *Config) *OIDCUserInfo {
	userInfo := &OIDCUserInfo{
		Groups: []string{},
	}

	userInfo.DisplayName = getClaimString(claims, config.nameClaim())
	userInfo.Email = getClaimString(claims, config.emailClaim())
	userInfo.AvatarURL = getClaimString(claims, config.avatarClaim())

	username := getClaimString(claims, config.usernameClaim())
	if username == "" {
		username = getClaimString(claims, "sub")
	}
	userInfo.Username = sanitizeUsername(username)

	userInfo.Groups = extractGroupsFromClaim(claims, config.groupsClaim())

	klog.V(4).InfoS("Extracted user info from OIDC token",
		"username", userInfo.Username,
		"groups", userInfo.Groups,
	)

	return userInfo
}

// getClaimString extracts a string value from a claims map.
func getClaimString(claims map[string]interface{}, key string) string {
	val, ok := claims[key]
	if !ok {
		return ""
	}

	switch v := val.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

// extractGroupsFromClaim extracts group claims from JWT using the configured claim name.
func extractGroupsFromClaim(claims map[string]interface{}, claimName string) []string {
	val, ok := claims[claimName]
	if !ok {
		return []string{}
	}

	switch v := val.(type) {
	case []interface{}:
		groups := make([]string, 0, len(v))
		for _, g := range v {
			if s, ok := g.(string); ok {
				groups = append(groups, s)
			}
		}
		return groups
	case string:
		parts := strings.Split(v, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	default:
		return []string{}
	}
}

// sanitizeUsername replaces characters problematic for Kubernetes usernames.
func sanitizeUsername(username string) string {
	username = strings.ReplaceAll(username, "@", "-")
	username = strings.ToLower(username)
	username = strings.TrimRight(username, ".-")
	return username
}

// OIDCUserInfo holds the extracted OIDC user identity.
type OIDCUserInfo struct {
	Username    string   `json:"username"`
	Groups      []string `json:"groups,omitempty"`
	DisplayName string   `json:"displayName,omitempty"`
	Email       string   `json:"email,omitempty"`
	AvatarURL   string   `json:"avatarUrl,omitempty"`
}
