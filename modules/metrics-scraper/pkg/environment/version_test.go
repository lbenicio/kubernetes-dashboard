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

package environment

import "testing"

func TestVersionAndUserAgent(t *testing.T) {
	previous := Version
	t.Cleanup(func() { Version = previous })
	Version = dev
	if !IsDev() || UserAgent() != "metrics-scraper:"+dev {
		t.Fatalf("development identity = %t/%q", IsDev(), UserAgent())
	}
	Version = "v1.2.3"
	if IsDev() || UserAgent() != "metrics-scraper:v1.2.3" {
		t.Fatalf("release identity = %t/%q", IsDev(), UserAgent())
	}
}
