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
	"crypto/tls"
	"net/http"
	"time"

	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	"github.com/go-openapi/spec"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog/v2"

	"k8s.io/dashboard/api/pkg/args"
	"k8s.io/dashboard/api/pkg/environment"
	"k8s.io/dashboard/api/pkg/handler"
	"k8s.io/dashboard/api/pkg/integration"
	integrationapi "k8s.io/dashboard/api/pkg/integration/api"
	"k8s.io/dashboard/certificates"
	"k8s.io/dashboard/certificates/ecdsa"
	"k8s.io/dashboard/client"
)

func main() {
	apiHandler, certs := initializeAPI()
	runAPI(apiHandler, certs, http.DefaultServeMux, handler.CreateAttachHandler("/api/sockjs"), promhttp.Handler(), args.InsecureAddress(), args.Address(), http.ListenAndServe, func(server *http.Server) error {
		return server.ListenAndServeTLS("", "")
	})

	select {}
}

func runAPI(
	apiHandler http.Handler,
	certs []tls.Certificate,
	mux *http.ServeMux,
	attachHandler, metricsHandler http.Handler,
	insecureAddress, secureAddress string,
	listenHTTP func(string, http.Handler) error,
	listenTLS func(*http.Server) error,
) {
	mux.Handle("/", apiHandler)
	mux.Handle("/api/sockjs/", attachHandler)
	mux.Handle("/metrics", metricsHandler)

	if len(certs) > 0 {
		klog.V(1).InfoS("Listening and serving on", "address", secureAddress)
		server := &http.Server{
			Addr:    secureAddress,
			Handler: mux,
			TLSConfig: &tls.Config{
				Certificates: certs,
				MinVersion:   tls.VersionTLS12,
			},
		}
		go func() {
			if err := listenTLS(server); err != nil {
				klog.Fatal(err)
			}
		}()
		return
	}

	klog.V(1).InfoS("Listening and serving on", "address", insecureAddress)
	go func() {
		if err := listenHTTP(insecureAddress, mux); err != nil {
			klog.Fatal(err)
		}
	}()
}

func initializeAPI() (*restful.Container, []tls.Certificate) {
	klog.InfoS("Starting Kubernetes Dashboard API", "version", environment.Version)

	client.Init(
		client.WithUserAgent(environment.UserAgent()),
		client.WithKubeconfig(args.KubeconfigPath()),
		client.WithMasterUrl(args.ApiServerHost()),
		client.WithInsecureTLSSkipVerify(args.ApiServerSkipTLSVerify()),
		client.WithCaBundle(args.ApiServerCaBundle()),
	)

	runWithProxyMode(args.IsProxyEnabled(), ensureAPIServerConnectionOrDie, func() {
		klog.Info("Running in proxy mode. InClusterClient connections will be disabled.")
	})

	// Init integrations
	integrationManager := integration.NewIntegrationManager()

	runWithProxyMode(args.IsProxyEnabled(), func() {
		configureMetricsProvider(integrationManager)
	}, func() {
		klog.Info("Skipping metrics configuration. Metrics not available in proxy mode.")
	})

	apiHandler, err := handler.CreateHTTPAPIHandler(integrationManager)
	if err != nil {
		handleFatalInitError(err)
	}

	if args.IsOpenAPIEnabled() {
		klog.Info("Enabling OpenAPI endpoint on /apidocs.json")
		configureOpenAPI(apiHandler)
	}

	certCreator := ecdsa.NewECDSACreator(args.KeyFile(), args.CertFile(), elliptic.P256())
	certManager := certificates.NewCertManager(certCreator, args.DefaultCertDir(), args.AutogenerateCertificates())
	certs, err := certManager.GetCertificates()
	if err != nil {
		handleFatalInitServingCertError(err)
	}
	return apiHandler, certs
}

func runWithProxyMode(proxyEnabled bool, inCluster, proxy func()) {
	if proxyEnabled {
		proxy()
		return
	}
	inCluster()
}

func ensureAPIServerConnectionOrDie() {
	versionInfo, err := client.InClusterClient().Discovery().ServerVersion()
	if err != nil {
		handleFatalInitError(err)
	}

	klog.InfoS("Successful initial request to the apiserver", "version", versionInfo.String())
}

func configureMetricsProvider(integrationManager integration.Manager) {
	switch metricsProvider := args.MetricsProvider(); metricsProvider {
	case "sidecar":
		integrationManager.Metric().ConfigureSidecar(args.SidecarHost()).
			EnableWithRetry(integrationapi.SidecarIntegrationID, time.Duration(args.MetricClientHealthCheckPeriod()))
	case "none":
		klog.Info("Metrics provider disabled")
	default:
		klog.InfoS("Invalid metrics provider", "provider", metricsProvider)
		klog.Info("Using default metrics provider", "provider", "sidecar")
		integrationManager.Metric().ConfigureSidecar(args.SidecarHost()).
			EnableWithRetry(integrationapi.SidecarIntegrationID, time.Duration(args.MetricClientHealthCheckPeriod()))
	}
}

func configureOpenAPI(container *restful.Container) {
	config := restfulspec.Config{
		WebServices:                   container.RegisteredWebServices(),
		APIPath:                       "/apidocs.json",
		PostBuildSwaggerObjectHandler: enrichOpenAPIObject,
	}
	container.Add(restfulspec.NewOpenAPIService(config))
}

func enrichOpenAPIObject(swo *spec.Swagger) {
	swo.Info = &spec.Info{
		InfoProps: spec.InfoProps{
			Title:   "Kubernetes Dashboard API",
			Version: environment.Version,
		},
	}
}

/**
 * Handles fatal init error that prevents server from doing any work. Prints verbose error
 * message and quits the server.
 */
func handleFatalInitError(err error) {
	klog.Fatalf("Error while initializing connection to Kubernetes apiserver. "+
		"This most likely means that the cluster is misconfigured (e.g., it has "+
		"invalid apiserver certificates or service account's configuration) or the "+
		"--apiserver-host param points to a server that does not exist. Reason: %s\n"+
		"Refer to our FAQ and wiki pages for more information: "+
		"https://github.com/kubernetes/dashboard/wiki/FAQ", err)
}

/**
 * Handles fatal init errors encountered during service cert loading.
 */
func handleFatalInitServingCertError(err error) {
	klog.Fatalf("Error while loading dashboard server certificates. Reason: %s", err)
}
