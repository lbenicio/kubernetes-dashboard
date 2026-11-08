// Copyright 2017 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"crypto/elliptic"

	"k8s.io/klog/v2"

	"k8s.io/dashboard/certificates"
	"k8s.io/dashboard/certificates/ecdsa"
	"k8s.io/dashboard/client"
	"k8s.io/dashboard/web/pkg/args"
	"k8s.io/dashboard/web/pkg/environment"
	"k8s.io/dashboard/web/pkg/router"

	// Importing route packages forces route registration
	_ "k8s.io/dashboard/web/pkg/config"
	_ "k8s.io/dashboard/web/pkg/locale"
	_ "k8s.io/dashboard/web/pkg/settings"
	_ "k8s.io/dashboard/web/pkg/systembanner"
)

func main() {
	runWebMain(initializeWebClient, getWebCertificatePaths, router.Router(), args.Address(), args.InsecureAddress(), klog.Fatalf)
}

type webHTTPServer interface {
	RunTLS(string, string, string) error
	Run(...string) error
}

func runWebMain(
	initializeClient func(),
	certificatePaths func() (string, string, error),
	server webHTTPServer,
	secureAddress, insecureAddress string,
	fatalf func(string, ...interface{}),
) {
	serveTLS := func(address, certPath, keyPath string) error {
		return server.RunTLS(address, certPath, keyPath)
	}
	serve := func(address string) error { return server.Run(address) }
	onFatal := func(initialized bool, err error) {
		if err == nil {
			return
		}
		if initialized {
			fatalf("Router error: %s", err)
			return
		}
		fatalf("Error while loading dashboard server certificates. Reason: %s", err)
	}
	runWebProcess(initializeClient, certificatePaths, serveTLS, serve, secureAddress, insecureAddress, onFatal)
}

func initializeWebClient() {
	client.Init(
		client.WithUserAgent(environment.UserAgent()),
		client.WithKubeconfig(args.KubeconfigPath()),
	)
}

func getWebCertificatePaths() (string, string, error) {
	certCreator := ecdsa.NewECDSACreator(args.KeyFile(), args.CertFile(), elliptic.P256())
	certManager := certificates.NewCertManager(certCreator, args.DefaultCertDir(), args.AutoGenerateCertificates())
	return certManager.GetCertificatePaths()
}

func runWebProcess(
	initializeClient func(),
	certificatePaths func() (string, string, error),
	serveTLS func(string, string, string) error,
	serve func(string) error,
	secureAddress, insecureAddress string,
	onFatal func(bool, error),
) {
	klog.InfoS("Starting Kubernetes Dashboard Web", "version", environment.Version)
	initialized, err := runWeb(initializeClient, certificatePaths, serveTLS, serve, secureAddress, insecureAddress)
	if err != nil {
		onFatal(initialized, err)
	}
}

func runWeb(
	initializeClient func(),
	certificatePaths func() (string, string, error),
	serveTLS func(string, string, string) error,
	serve func(string) error,
	secureAddress, insecureAddress string,
) (bool, error) {
	initializeClient()
	certPath, keyPath, err := certificatePaths()
	if err != nil {
		return false, err
	}

	if len(certPath) != 0 && len(keyPath) != 0 {
		klog.V(1).InfoS("Listening and serving securely on", "address", secureAddress)
		return true, serveTLS(secureAddress, certPath, keyPath)
	}

	klog.V(1).InfoS("Listening and serving insecurely on", "address", insecureAddress)
	return true, serve(insecureAddress)
}
