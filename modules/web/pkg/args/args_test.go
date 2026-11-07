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

	"k8s.io/dashboard/certificates/api"
)

func TestArgumentAccessors(t *testing.T) {
	old := struct {
		autoGenerate bool
		insecurePort int
		port         int
		insecureIP   string
		bindIP       string
		namespace    string
		certDir      string
		certFile     string
		keyFile      string
		configMap    string
		banner       string
		severity     string
		locale       string
		kubeconfig   string
	}{*argAutoGenerateCertificates, *argInsecurePort, *argPort, argInsecureBindAddress.String(), argBindAddress.String(), *argNamespace, *argDefaultCertDir, *argCertFile, *argKeyFile, *argSettingsConfigMapName, *argSystemBanner, *argSystemBannerSeverity, *argLocaleConfig, *argKubeconfig}
	t.Cleanup(func() {
		*argAutoGenerateCertificates = old.autoGenerate
		*argInsecurePort = old.insecurePort
		*argPort = old.port
		*argInsecureBindAddress = net.ParseIP(old.insecureIP)
		*argBindAddress = net.ParseIP(old.bindIP)
		*argNamespace = old.namespace
		*argDefaultCertDir = old.certDir
		*argCertFile = old.certFile
		*argKeyFile = old.keyFile
		*argSettingsConfigMapName = old.configMap
		*argSystemBanner = old.banner
		*argSystemBannerSeverity = old.severity
		*argLocaleConfig = old.locale
		*argKubeconfig = old.kubeconfig
	})

	*argAutoGenerateCertificates = false
	*argInsecurePort, *argPort = 8100, 8101
	*argInsecureBindAddress, *argBindAddress = net.ParseIP("127.0.0.2"), net.ParseIP("127.0.0.3")
	*argNamespace, *argDefaultCertDir = "dashboard-test", "/tmp/certs"
	*argCertFile, *argKeyFile = "custom.crt", "custom.key"
	*argSettingsConfigMapName = "settings-test"
	*argSystemBanner, *argSystemBannerSeverity = "Maintenance", "WARNING"
	*argLocaleConfig, *argKubeconfig = "/tmp/locales.json", "/tmp/kubeconfig"

	if Namespace() != "dashboard-test" || InsecurePort() != 8100 || Port() != 8101 {
		t.Fatal("namespace or port accessors returned unexpected values")
	}
	if !InsecureBindAddress().Equal(net.ParseIP("127.0.0.2")) || !BindAddress().Equal(net.ParseIP("127.0.0.3")) {
		t.Fatal("bind address accessors returned unexpected values")
	}
	if DefaultCertDir() != "/tmp/certs" || CertFile() != "custom.crt" || KeyFile() != "custom.key" {
		t.Fatal("certificate accessors returned unexpected values")
	}
	if SettingsConfigMapName() != "settings-test" || SystemBanner() != "Maintenance" || SystemBannerSeverity() != "WARNING" {
		t.Fatal("settings or system banner accessors returned unexpected values")
	}
	if LocaleConfig() != "/tmp/locales.json" || KubeconfigPath() != "/tmp/kubeconfig" {
		t.Fatal("locale or kubeconfig accessors returned unexpected values")
	}
	if InsecureAddress() != "127.0.0.2:8100" || Address() != "127.0.0.3:8101" {
		t.Fatal("address accessors returned unexpected values")
	}

	*argAutoGenerateCertificates = true
	*argCertFile, *argKeyFile = "", ""
	if CertFile() != api.DashboardCertName || KeyFile() != api.DashboardKeyName {
		t.Fatal("certificate filenames were not generated when certificate generation is enabled")
	}
}
