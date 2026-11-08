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
	"os"

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
	_ "k8s.io/dashboard/auth/pkg/routes/oidc"
)

func main() {
	runAuthProcess(initializeAuth, func(address string) error {
		return router.Router().Run(address)
	}, args.Address(), func(int) { os.Exit(1) })
}

func runAuthProcess(initialize func() error, serve func(string) error, address string, exit func(int)) {
	klog.InfoS("Starting Kubernetes Dashboard Auth", "version", environment.Version)
	initialized, err := runAuth(initialize, serve, address)
	if err != nil {
		if initialized {
			klog.ErrorS(err, "Router error")
		} else {
			klog.ErrorS(err, "Could not initialize OpenID Connect")
		}
		exit(1)
	}
}

func runAuth(initialize func() error, serve func(string) error, address string) (bool, error) {
	if err := initialize(); err != nil {
		return false, err
	}

	klog.V(1).InfoS("Listening and serving insecurely on", "address", address)
	if err := serve(address); err != nil {
		return true, err
	}
	return true, nil
}

func initializeAuth() error {
	client.SetSessionCookieSecure(args.SessionCookieSecure())

	client.Init(
		client.WithUserAgent(environment.UserAgent()),
		client.WithKubeconfig(args.KubeconfigPath()),
		client.WithMasterUrl(args.ApiServerHost()),
		client.WithInsecureTLSSkipVerify(args.ApiServerSkipTLSVerify()),
		client.WithCaBundle(args.ApiServerCaBundle()),
	)
	return oidc.Initialize()
}
