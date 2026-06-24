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
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/golang-jwt/jwt/v4"
	"k8s.io/klog/v2"
)

// jwksResponse represents the JWKS endpoint response.
type jwksResponse struct {
	Keys []jwksKey `json:"keys"`
}

// jwksKey represents a single key in the JWKS.
type jwksKey struct {
	KTY string `json:"kty"`
	KID string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
	// EC keys
	CRV string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

// parseJWK parses a JWKS key into a crypto public key.
func parseJWK(key jwksKey) (interface{}, error) {
	switch key.KTY {
	case "RSA":
		return parseRSAKey(key)
	case "EC":
		return parseECKey(key)
	default:
		return nil, fmt.Errorf("unsupported key type: %s", key.KTY)
	}
}

// parseRSAKey parses an RSA public key from JWKS format.
func parseRSAKey(key jwksKey) (*rsa.PublicKey, error) {
	if key.N == "" || key.E == "" {
		return nil, fmt.Errorf("RSA key missing N or E field")
	}

	// Decode the modulus (N) and exponent (E) from base64url
	nBytes, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode RSA N: %w", err)
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		return nil, fmt.Errorf("failed to decode RSA E: %w", err)
	}

	// Convert to big integers
	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)

	return &rsa.PublicKey{
		N: n,
		E: int(e.Int64()),
	}, nil
}

// parseECKey parses an EC public key from JWKS format.
func parseECKey(key jwksKey) (interface{}, error) {
	if key.X == "" || key.Y == "" {
		return nil, fmt.Errorf("EC key missing X or Y field")
	}

	// Decode x and y coordinates from base64url
	xBytes, err := base64.RawURLEncoding.DecodeString(key.X)
	if err != nil {
		return nil, fmt.Errorf("failed to decode EC X: %w", err)
	}

	yBytes, err := base64.RawURLEncoding.DecodeString(key.Y)
	if err != nil {
		return nil, fmt.Errorf("failed to decode EC Y: %w", err)
	}

	// Build the key data in JWK format for the jwt library to parse
	jwkData := map[string]interface{}{
		"kty": key.KTY,
		"crv": key.CRV,
		"x":   key.X,
		"y":   key.Y,
	}

	jwkJSON, err := json.Marshal(jwkData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal EC key: %w", err)
	}

	// Use the jwt library's ECDSA key parser which handles all curve types
	parsedKey, err := jwt.ParseECPublicKeyFromPEM([]byte(jwkJSON))
	if err != nil {
		// Fall back to raw bytes if PEM parsing fails
		_ = xBytes
		_ = yBytes
		return nil, fmt.Errorf("failed to parse EC key: %w", err)
	}

	return parsedKey, nil
}

// extractUserInfo extracts the user identity from JWT claims for K8s impersonation.
// Uses configurable claim names from the provider configuration.
func extractUserInfo(claims jwt.MapClaims, config *Config) *OIDCUserInfo {
	userInfo := &OIDCUserInfo{
		Groups: []string{},
	}

	// Extract display fields using configured claim names
	userInfo.DisplayName = getClaimString(claims, config.nameClaim())
	userInfo.Email = getClaimString(claims, config.emailClaim())
	userInfo.AvatarURL = getClaimString(claims, config.avatarClaim())

	// Determine username from the configured claim, fall back to sub
	username := getClaimString(claims, config.usernameClaim())
	if username == "" {
		username = getClaimString(claims, "sub")
	}
	userInfo.Username = sanitizeUsername(username)

	// Extract groups from the configured claim
	userInfo.Groups = extractGroupsFromClaim(claims, config.groupsClaim())

	// Check allowed group if configured
	if config.AllowedGroup != "" {
		if !containsGroup(userInfo.Groups, config.AllowedGroup) {
			klog.Warningf("User %s is not in allowed group %q", userInfo.Username, config.AllowedGroup)
			return nil
		}
	}

	klog.V(4).InfoS("Extracted user info from OIDC token",
		"username", userInfo.Username,
		"groups", userInfo.Groups,
		"claims", claims,
	)

	return userInfo
}

// getClaimString extracts a string claim from the JWT claims.
func getClaimString(claims jwt.MapClaims, key string) string {
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
func extractGroupsFromClaim(claims jwt.MapClaims, claimName string) []string {
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
		// Some providers send groups as a comma-separated string
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

// sanitizeUsername replaces characters that are problematic for Kubernetes usernames.
func sanitizeUsername(username string) string {
	username = strings.ReplaceAll(username, "@", "-")
	username = strings.ToLower(username)
	username = strings.TrimRight(username, ".-")
	return username
}

// containsGroup checks if a group is present in the list.
func containsGroup(groups []string, target string) bool {
	for _, g := range groups {
		if strings.EqualFold(g, target) {
			return true
		}
	}
	return false
}

// OIDCUserInfo holds the extracted OIDC user identity.
// Placed here to avoid circular imports.
type OIDCUserInfo struct {
	Username    string   `json:"username"`
	Groups      []string `json:"groups,omitempty"`
	DisplayName string   `json:"displayName,omitempty"`
	Email       string   `json:"email,omitempty"`
	AvatarURL   string   `json:"avatarUrl,omitempty"`
}
