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

package ecdsa_test

import (
	"crypto/elliptic"
	"crypto/x509"
	"net"
	"testing"

	"k8s.io/dashboard/certificates/ecdsa"
)

func TestNewECDSACreator(t *testing.T) {
	keyFile := "cert.key"
	certFile := "cert.crt"
	creator := ecdsa.NewECDSACreator(keyFile, certFile, elliptic.P256())

	if creator == nil {
		t.Fatal("Expected creator not to be nil.")
	}
}

func TestEcdsaCreator_GetCertFileName(t *testing.T) {
	keyFile := "cert.key"
	certFile := "cert.crt"
	creator := ecdsa.NewECDSACreator(keyFile, certFile, elliptic.P256())

	if creator.GetCertFileName() != certFile {
		t.Fatalf("Expected cert file name to equal %s but got %s.", certFile, creator.GetCertFileName())
	}
}

func TestEcdsaCreator_GetKeyFileName(t *testing.T) {
	keyFile := "cert.key"
	certFile := "cert.crt"
	creator := ecdsa.NewECDSACreator(keyFile, certFile, elliptic.P256())

	if creator.GetKeyFileName() != keyFile {
		t.Fatalf("Expected cert key file name to equal %s but got %s.", keyFile, creator.GetKeyFileName())
	}
}

func TestEcdsaCreatorGeneratesCertificateForDashboardPod(t *testing.T) {
	t.Setenv("POD_NAME", "dashboard-0")
	t.Setenv("POD_NAMESPACE", "kubernetes-dashboard")
	t.Setenv("POD_IP", "192.0.2.10")
	creator := ecdsa.NewECDSACreator("", "", elliptic.P256())
	certificateBytes := creator.GenerateCertificate(creator.GenerateKey())
	certificate, err := x509.ParseCertificate(certificateBytes)
	if err != nil {
		t.Fatalf("ParseCertificate() error = %v", err)
	}
	if len(certificate.DNSNames) != 1 || certificate.DNSNames[0] != "dashboard-0.kubernetes-dashboard" {
		t.Errorf("certificate DNS names = %v", certificate.DNSNames)
	}
	if len(certificate.IPAddresses) != 1 || !certificate.IPAddresses[0].Equal(net.ParseIP("192.0.2.10")) {
		t.Errorf("certificate IP addresses = %v", certificate.IPAddresses)
	}
}

func TestEcdsaCreatorUsesDefaultIdentityWithoutPodMetadata(t *testing.T) {
	t.Setenv("POD_NAME", "")
	t.Setenv("POD_NAMESPACE", "")
	t.Setenv("POD_IP", "")
	creator := ecdsa.NewECDSACreator("", "", elliptic.P256())
	certificateBytes := creator.GenerateCertificate(creator.GenerateKey())
	certificate, err := x509.ParseCertificate(certificateBytes)
	if err != nil {
		t.Fatalf("ParseCertificate() error = %v", err)
	}
	if certificate.Subject.CommonName != "kubernetes-dashboard" || len(certificate.DNSNames) != 0 || len(certificate.IPAddresses) != 0 {
		t.Errorf("default certificate identity = subject %q, DNS %v, IPs %v", certificate.Subject.CommonName, certificate.DNSNames, certificate.IPAddresses)
	}
}
