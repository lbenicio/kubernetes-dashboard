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

package login

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	v1 "k8s.io/dashboard/auth/api/v1"
	"k8s.io/dashboard/auth/pkg/args"
	"k8s.io/dashboard/auth/pkg/oidc"
	"k8s.io/dashboard/auth/pkg/router"
	"k8s.io/dashboard/client"
)

func init() {
	router.V1().POST("/login", handleLogin)
	router.V1().GET("/login/config", handleLoginConfig)
	router.V1().POST("/logout", handleLogout)
}

func handleLogin(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if args.AnonymousAdminEnabled() {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	loginRequest := new(v1.LoginRequest)
	err := c.Bind(loginRequest)
	if err != nil {
		klog.ErrorS(err, "Could not read login request")
		c.JSON(http.StatusBadRequest, err)
		return
	}

	response, code, err := login(loginRequest, c.Request)
	if err != nil {
		klog.ErrorS(err, "Could not log in")
		c.JSON(code, err)
		return
	}
	if client.HasSessionKey() {
		if err := client.SetBrowserSessionCookie(c.Writer, c.Request, client.BrowserSession{
			KubernetesToken: response.Token,
			ExpiresAt:       time.Now().Add(12 * time.Hour),
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create a secure login session"})
			return
		}
		response.Token = ""
	}

	c.JSON(code, response)
}

func handleLoginConfig(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"oidcEnabled":    oidc.Enabled(),
		"anonymousAdmin": args.AnonymousAdminEnabled(),
	})
}

func handleLogout(c *gin.Context) {
	client.ClearBrowserSessionCookie(c.Writer, c.Request)
	client.ClearProtectedCookie(c.Writer, c.Request, "kd-oidc-state")
	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusNoContent)
}
