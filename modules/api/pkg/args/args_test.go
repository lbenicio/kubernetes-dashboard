// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

package args

import (
	"net"
	"testing"

	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
)

func TestArgumentAccessors(t *testing.T) {
	old := struct {
		csrf, proxy, openAPI, skipTLS, autoCert bool
		insecurePort, port, metricPeriod        int
		insecureIP, bindIP                      string
		certDir, caBundle, certFile, keyFile    string
		apiHost, metricsProvider, sidecarHost   string
		kubeconfig, namespace, scraperName      string
	}{
		*argDisableCSRFProtection, *argIsProxyEnabled, *argOpenAPIEnabled, *argApiServerSkipTLSVerify, *argAutoGenerateCertificates,
		*argInsecurePort, *argPort, *argMetricClientCheckPeriod, argInsecureBindAddress.String(), argBindAddress.String(),
		*argDefaultCertDir, *argApiServerCaBundle, *argCertFile, *argKeyFile, *argApiServerHost, *argMetricsProvider,
		*argSidecarHost, *argKubeConfigFile, *argNamespace, *argMetricsScraperServiceName,
	}
	logLevel := pflag.Lookup("v").Value.String()
	t.Cleanup(func() {
		*argDisableCSRFProtection, *argIsProxyEnabled, *argOpenAPIEnabled = old.csrf, old.proxy, old.openAPI
		*argApiServerSkipTLSVerify, *argAutoGenerateCertificates = old.skipTLS, old.autoCert
		*argInsecurePort, *argPort, *argMetricClientCheckPeriod = old.insecurePort, old.port, old.metricPeriod
		*argInsecureBindAddress, *argBindAddress = net.ParseIP(old.insecureIP), net.ParseIP(old.bindIP)
		*argDefaultCertDir, *argApiServerCaBundle, *argCertFile, *argKeyFile = old.certDir, old.caBundle, old.certFile, old.keyFile
		*argApiServerHost, *argMetricsProvider, *argSidecarHost = old.apiHost, old.metricsProvider, old.sidecarHost
		*argKubeConfigFile, *argNamespace, *argMetricsScraperServiceName = old.kubeconfig, old.namespace, old.scraperName
		_ = pflag.Lookup("v").Value.Set(logLevel)
	})

	*argDisableCSRFProtection, *argIsProxyEnabled, *argOpenAPIEnabled = true, true, true
	*argApiServerSkipTLSVerify, *argAutoGenerateCertificates = true, true
	*argInsecurePort, *argPort, *argMetricClientCheckPeriod = 8002, 8444, 15
	*argInsecureBindAddress, *argBindAddress = net.ParseIP("127.0.0.2"), net.ParseIP("127.0.0.3")
	*argDefaultCertDir, *argApiServerCaBundle, *argCertFile, *argKeyFile = "/tmp/certs", "/tmp/ca.pem", "tls.crt", "tls.key"
	*argApiServerHost, *argMetricsProvider, *argSidecarHost = "https://api.test", "prometheus", "http://metrics.test"
	*argKubeConfigFile, *argNamespace, *argMetricsScraperServiceName = "/tmp/kubeconfig", "dashboard-test", "scraper-test"
	if err := pflag.Lookup("v").Value.Set("4"); err != nil {
		t.Fatal(err)
	}

	if Address() != "127.0.0.3:8444" || InsecureAddress() != "127.0.0.2:8002" {
		t.Fatal("address accessors returned unexpected values")
	}
	if DefaultCertDir() != "/tmp/certs" || ApiServerCaBundle() != "/tmp/ca.pem" || CertFile() != "tls.crt" || KeyFile() != "tls.key" {
		t.Fatal("certificate accessors returned unexpected values")
	}
	if ApiServerHost() != "https://api.test" || MetricsProvider() != "prometheus" || SidecarHost() != "http://metrics.test" {
		t.Fatal("API or metrics accessors returned unexpected values")
	}
	if KubeconfigPath() != "/tmp/kubeconfig" || Namespace() != "dashboard-test" || MetricsScraperServiceName() != "scraper-test" {
		t.Fatal("Kubernetes accessors returned unexpected values")
	}
	if !ApiServerSkipTLSVerify() || !AutogenerateCertificates() || !IsProxyEnabled() || !IsOpenAPIEnabled() || IsCSRFProtectionEnabled() {
		t.Fatal("boolean accessors returned unexpected values")
	}
	if MetricClientHealthCheckPeriod() != 15 || APILogLevel() != klog.Level(4) {
		t.Fatal("period or log-level accessor returned an unexpected value")
	}
}
