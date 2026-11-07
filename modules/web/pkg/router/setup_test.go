// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

package router

import "testing"

func TestRouterAccessors(t *testing.T) {
	if Root() == nil {
		t.Fatal("Root() returned nil")
	}
	if Router() == nil {
		t.Fatal("Router() returned nil")
	}
}
