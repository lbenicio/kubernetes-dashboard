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
	"net/http"

	"github.com/gin-gonic/gin"

	"k8s.io/dashboard/auth/pkg/router"
	oidcpkg "k8s.io/dashboard/auth/pkg/oidc"
)

// Provider is the global OIDC provider instance, initialized in main.go.
var Provider *oidcpkg.Provider

func init() {
	router.V1().GET("/oidc/config", handleGetConfig)
	router.V1().GET("/oidc/login", handleLogin)
	router.V1().GET("/oidc/callback", handleCallback)
	router.V1().GET("/oidc/session", handleSessionInfo)
	router.V1().POST("/oidc/refresh", handleRefresh)
	router.V1().POST("/oidc/logout", handleLogout)
}

func handleGetConfig(c *gin.Context) {
	if Provider == nil {
		c.JSON(http.StatusOK, gin.H{"enabled": false})
		return
	}
	HandleGetConfig(Provider)(c.Writer, c.Request)
}

func handleLogin(c *gin.Context) {
	if Provider == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OIDC not configured"})
		return
	}
	HandleLogin(Provider)(c.Writer, c.Request)
}

func handleCallback(c *gin.Context) {
	if Provider == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OIDC not configured"})
		return
	}
	HandleCallback(Provider)(c.Writer, c.Request)
}

func handleSessionInfo(c *gin.Context) {
	if Provider == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"authenticated": false})
		return
	}
	HandleSessionInfo(Provider)(c.Writer, c.Request)
}

func handleRefresh(c *gin.Context) {
	if Provider == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OIDC not configured"})
		return
	}
	HandleRefresh(Provider)(c.Writer, c.Request)
}

func handleLogout(c *gin.Context) {
	if Provider == nil {
		c.JSON(http.StatusOK, gin.H{"message": "logged out"})
		return
	}
	HandleLogout(Provider)(c.Writer, c.Request)
}
