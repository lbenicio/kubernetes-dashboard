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

package main

import (
	"context"
	"os"
	"time"

	"k8s.io/klog/v2"

	"k8s.io/dashboard/auth/pkg/args"
	"k8s.io/dashboard/auth/pkg/environment"
	"k8s.io/dashboard/auth/pkg/oidc"
	"k8s.io/dashboard/auth/pkg/router"
	"k8s.io/dashboard/client"

	// Importing route packages forces route registration
	_ "k8s.io/dashboard/auth/pkg/routes/csrftoken"
	_ "k8s.io/dashboard/auth/pkg/routes/login"
	_ "k8s.io/dashboard/auth/pkg/routes/me"
	oidcRoutes "k8s.io/dashboard/auth/pkg/routes/oidc"
)

func main() {
	klog.InfoS("Starting Kubernetes Dashboard Auth", "version", environment.Version)

	client.Init(
		client.WithUserAgent(environment.UserAgent()),
		client.WithKubeconfig(args.KubeconfigPath()),
		client.WithMasterUrl(args.ApiServerHost()),
		client.WithInsecureTLSSkipVerify(args.ApiServerSkipTLSVerify()),
		client.WithCaBundle(args.ApiServerCaBundle()),
	)

	// Initialize OIDC provider if configured
	initOIDC()

	klog.V(1).InfoS("Listening and serving insecurely on", "address", args.Address())
	if err := router.Router().Run(args.Address()); err != nil {
		klog.ErrorS(err, "Router error")
		os.Exit(1)
	}
}

func initOIDC() {
	oidcConfig := &oidc.Config{
		IssuerURL:    args.OIDCIssuerURL(),
		ClientID:     args.OIDCClientID(),
		ClientSecret: args.OIDCClientSecret(),
		RedirectURL:  args.OIDCRedirectURL(),
		Scopes:       args.OIDCScopes(),
		CookieSecret:        args.OIDCCookieSecret(),
		ProviderName:        args.OIDCProviderName(),
		InsecureSkipVerify:  args.OIDCSkipTLSVerify(),
		CABundle:            args.OIDCCABundle(),
		UsernameClaim:       args.OIDCUsernameClaim(),
		GroupsClaim:         args.OIDCGroupsClaim(),
		AvatarClaim:         args.OIDCAvatarClaim(),
		NameClaim:           args.OIDCNameClaim(),
		EmailClaim:          args.OIDCEmailClaim(),
		AllowedGroup:        args.OIDCAllowedGroup(),
	}

	if !oidcConfig.IsEnabled() {
		klog.InfoS("OIDC is not configured. Token-based authentication will be used.")
		return
	}

	provider := oidc.NewProvider(oidcConfig)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := provider.Initialize(ctx); err != nil {
		klog.ErrorS(err, "Failed to initialize OIDC provider. OIDC authentication will be disabled.")
		return
	}

	// Set the global provider for routes to use
	oidcRoutes.Provider = provider

	klog.InfoS("OIDC authentication enabled", "provider", oidcConfig.ProviderName)
}
