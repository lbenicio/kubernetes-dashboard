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
