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

package ecdsa

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestECDSACreatorGeneratesPodCertificate(t *testing.T) {
	t.Setenv("POD_NAME", "dashboard-web")
	t.Setenv("POD_NAMESPACE", "dashboard")
	t.Setenv("POD_IP", "10.1.2.3")
	creator := NewECDSACreator("key.pem", "cert.pem", elliptic.P256()).(*ecdsaCreator)
	key := creator.GenerateKey()
	der := creator.GenerateCertificate(key)
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("ParseCertificate() error = %v", err)
	}
	if certificate.Subject.CommonName != "dashboard-web.dashboard" || len(certificate.DNSNames) != 1 || certificate.DNSNames[0] != "dashboard-web.dashboard" {
		t.Fatalf("pod certificate identity = %#v", certificate.Subject)
	}
	if len(certificate.IPAddresses) != 1 || !certificate.IPAddresses[0].Equal(net.ParseIP("10.1.2.3")) {
		t.Fatalf("pod certificate IP SANs = %v", certificate.IPAddresses)
	}

	keyPEM, certPEM, err := creator.KeyCertPEMBytes(key, der)
	if err != nil {
		t.Fatalf("KeyCertPEMBytes() error = %v", err)
	}
	if _, err := tls.X509KeyPair(certPEM, keyPEM); err != nil {
		t.Fatalf("generated PEM pair is invalid: %v", err)
	}

	certPath, keyPath := creator.StoreCertificates(t.TempDir(), key, der)
	if filepath.Base(certPath) != "cert.pem" || filepath.Base(keyPath) != "key.pem" {
		t.Fatalf("stored certificate paths = %q, %q", certPath, keyPath)
	}
	if _, err := os.Stat(certPath); err != nil {
		t.Errorf("stored certificate is missing: %v", err)
	}
	if _, err := os.Stat(keyPath); err != nil {
		t.Errorf("stored key is missing: %v", err)
	}
}

func TestECDSACreatorDefaultsAndFallbackSubject(t *testing.T) {
	t.Setenv("POD_NAME", "")
	t.Setenv("POD_NAMESPACE", "")
	t.Setenv("POD_IP", "")
	creator := NewECDSACreator("", "", elliptic.P256()).(*ecdsaCreator)
	if creator.GetCertFileName() != "dashboard.crt" || creator.GetKeyFileName() != "dashboard.key" {
		t.Fatalf("default certificate filenames = %q, %q", creator.GetCertFileName(), creator.GetKeyFileName())
	}
	key := creator.GenerateKey()
	certificate, err := x509.ParseCertificate(creator.GenerateCertificate(key))
	if err != nil {
		t.Fatalf("ParseCertificate() error = %v", err)
	}
	if certificate.Subject.CommonName != "kubernetes-dashboard" || len(certificate.DNSNames) != 0 || len(certificate.IPAddresses) != 0 {
		t.Fatalf("fallback certificate identity = %#v, DNS=%v IP=%v", certificate.Subject, certificate.DNSNames, certificate.IPAddresses)
	}
}

func TestECDSACreatorKeyPEMRejectsInvalidKey(t *testing.T) {
	creator := NewECDSACreator("key.pem", "cert.pem", elliptic.P256()).(*ecdsaCreator)
	if _, _, err := creator.KeyCertPEMBytes(&ecdsa.PrivateKey{}, nil); err == nil {
		t.Fatal("KeyCertPEMBytes() accepted an invalid private key")
	}
}

func TestECDSACreatorFatalPaths(t *testing.T) {
	if fatalPath := os.Getenv("DASHBOARD_ECDSA_FATAL_PATH"); fatalPath != "" {
		creator := NewECDSACreator("key.pem", "cert.pem", elliptic.P256()).(*ecdsaCreator)
		switch fatalPath {
		case "generate-key":
			creator.keyGenerator = func(elliptic.Curve, io.Reader) (*ecdsa.PrivateKey, error) {
				return nil, errors.New("key generation failed")
			}
			creator.GenerateKey()
		case "generate-certificate":
			creator.GenerateCertificate(&ecdsa.PrivateKey{})
		case "store-certificate":
			creator.StoreCertificates(filepath.Join(os.Getenv("DASHBOARD_ECDSA_TEST_DIR"), "missing"), creator.GenerateKey(), []byte("der"))
		case "store-key":
			creator = NewECDSACreator("missing/key.pem", "cert.pem", elliptic.P256()).(*ecdsaCreator)
			creator.StoreCertificates(os.Getenv("DASHBOARD_ECDSA_TEST_DIR"), creator.GenerateKey(), []byte("der"))
		case "invalid-key-type":
			creator.getKey("not an ECDSA key")
		case "serial-number":
			rand.Reader = failingRandomReader{}
			creator.generateSerialNumber()
		}
		t.Fatal("fatal path returned instead of terminating the process")
	}

	for _, fatalPath := range []string{"generate-key", "generate-certificate", "store-certificate", "store-key", "invalid-key-type", "serial-number"} {
		t.Run(fatalPath, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestECDSACreatorFatalPaths$")
			cmd.Env = append(os.Environ(), "DASHBOARD_ECDSA_FATAL_PATH="+fatalPath, "DASHBOARD_ECDSA_TEST_DIR="+t.TempDir())
			output, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(output), "ECDSAManager") {
				t.Fatalf("fatal path %q returned %v with output %q", fatalPath, err, output)
			}
		})
	}
}

type failingRandomReader struct{}

func (failingRandomReader) Read([]byte) (int, error) { return 0, errors.New("random source failed") }
