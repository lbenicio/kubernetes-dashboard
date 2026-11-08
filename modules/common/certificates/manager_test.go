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

package certificates

import (
	"crypto/elliptic"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/dashboard/certificates/api"
	"k8s.io/dashboard/certificates/ecdsa"
)

type certificateCreatorWithPEMError struct {
	api.Creator
}

func (certificateCreatorWithPEMError) GenerateKey() interface{} { return struct{}{} }

func (certificateCreatorWithPEMError) GenerateCertificate(interface{}) []byte { return nil }

func (certificateCreatorWithPEMError) KeyCertPEMBytes(interface{}, []byte) ([]byte, []byte, error) {
	return nil, nil, errors.New("certificate serialization failed")
}

func (certificateCreatorWithPEMError) StoreCertificates(string, interface{}, []byte) (string, string) {
	return "", ""
}

func (certificateCreatorWithPEMError) GetKeyFileName() string { return "key" }

func (certificateCreatorWithPEMError) GetCertFileName() string { return "cert" }

func TestCertificateManagerAutogenerate(t *testing.T) {
	t.Setenv("POD_NAME", "dashboard-test")
	t.Setenv("POD_NAMESPACE", "dashboard")
	t.Setenv("POD_IP", "127.0.0.1")
	certDir := t.TempDir()
	creator := ecdsa.NewECDSACreator("", "", elliptic.P256())
	manager := NewCertManager(creator, certDir, true)

	certificates, err := manager.GetCertificates()
	if err != nil || len(certificates) != 1 || len(certificates[0].Certificate) == 0 {
		t.Fatalf("GetCertificates() = %d certificates, %v", len(certificates), err)
	}

	certPath, keyPath, err := manager.GetCertificatePaths()
	if err != nil {
		t.Fatalf("GetCertificatePaths() error = %v", err)
	}
	if certPath != filepath.Join(certDir, api.DashboardCertName) || keyPath != filepath.Join(certDir, api.DashboardKeyName) {
		t.Fatalf("certificate paths = %q, %q", certPath, keyPath)
	}
	if _, err := os.Stat(certPath); err != nil {
		t.Errorf("generated certificate is unavailable: %v", err)
	}
	if _, err := os.Stat(keyPath); err != nil {
		t.Errorf("generated key is unavailable: %v", err)
	}
}

func TestCertificateManagerLoadsExistingFiles(t *testing.T) {
	certDir := t.TempDir()
	creator := ecdsa.NewECDSACreator("", "", elliptic.P256())
	manager := NewCertManager(creator, certDir, true)
	if _, _, err := manager.GetCertificatePaths(); err != nil {
		t.Fatalf("create test certificate pair: %v", err)
	}

	manager = NewCertManager(creator, certDir, false)
	certificates, err := manager.GetCertificates()
	if err != nil || len(certificates) != 1 {
		t.Fatalf("GetCertificates() = %d certificates, %v", len(certificates), err)
	}
	certPath, keyPath, err := manager.GetCertificatePaths()
	if err != nil || certPath != filepath.Join(certDir, api.DashboardCertName) || keyPath != filepath.Join(certDir, api.DashboardKeyName) {
		t.Fatalf("GetCertificatePaths() = %q, %q, %v", certPath, keyPath, err)
	}
}

func TestCertificateManagerReturnsNoCertificatesWhenFilesAreMissing(t *testing.T) {
	manager := NewCertManager(ecdsa.NewECDSACreator("", "", elliptic.P256()), t.TempDir(), false)
	certificates, err := manager.GetCertificates()
	if err != nil || certificates != nil {
		t.Fatalf("GetCertificates() = %#v, %v, want nil, nil", certificates, err)
	}
	certPath, keyPath, err := manager.GetCertificatePaths()
	if err != nil || certPath != "" || keyPath != "" {
		t.Fatalf("GetCertificatePaths() = %q, %q, %v, want empty paths", certPath, keyPath, err)
	}
}

func TestCertificateManagerReportsInvalidExistingPair(t *testing.T) {
	certDir := t.TempDir()
	creator := ecdsa.NewECDSACreator("", "", elliptic.P256())
	for _, name := range []string{creator.GetCertFileName(), creator.GetKeyFileName()} {
		if err := os.WriteFile(filepath.Join(certDir, name), []byte("invalid"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	manager := NewCertManager(creator, certDir, false)
	if _, err := manager.GetCertificates(); err == nil {
		t.Fatal("GetCertificates() accepted an invalid certificate pair")
	}
}

func TestCertificateManagerReturnsCertificateSerializationErrors(t *testing.T) {
	creator := certificateCreatorWithPEMError{}
	manager := NewCertManager(creator, t.TempDir(), true)
	if _, err := manager.GetCertificates(); err == nil || err.Error() != "certificate serialization failed" {
		t.Fatalf("GetCertificates() error = %v, want certificate serialization failure", err)
	}
	if certPath, keyPath, err := manager.GetCertificatePaths(); err == nil || err.Error() != "certificate serialization failed" || certPath != "" || keyPath != "" {
		t.Fatalf("GetCertificatePaths() = %q, %q, %v; want serialization error", certPath, keyPath, err)
	}
}
