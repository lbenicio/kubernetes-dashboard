// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

package environment

import "testing"

func TestVersionHelpers(t *testing.T) {
	previous := Version
	t.Cleanup(func() { Version = previous })

	Version = dev
	if !IsDev() || UserAgent() != "dashboard-web:0.0.0-dev" {
		t.Fatalf("development version helpers returned IsDev=%t UserAgent=%q", IsDev(), UserAgent())
	}

	Version = "7.14.10"
	if IsDev() || UserAgent() != "dashboard-web:7.14.10" {
		t.Fatalf("release version helpers returned IsDev=%t UserAgent=%q", IsDev(), UserAgent())
	}
}
