// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

package systembanner

import "testing"

func TestToSystemBannerSeverity(t *testing.T) {
	cases := []struct {
		input string
		want  SystemBannerSeverity
	}{
		{input: "INFO", want: SystemBannerSeverityInfo},
		{input: "WARNING", want: SystemBannerSeverityWarning},
		{input: "ERROR", want: SystemBannerSeverityError},
		{input: "unknown", want: SystemBannerSeverityInfo},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			if got := toSystemBannerSeverity(tc.input); got != tc.want {
				t.Errorf("toSystemBannerSeverity(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
