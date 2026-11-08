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
	"testing"
	"time"

	"github.com/spf13/pflag"
)

func TestMetricScraperArgumentAccessors(t *testing.T) {
	setMetricFlag(t, "kubeconfig", "/tmp/metrics-kubeconfig")
	setMetricFlag(t, "db-file", "/tmp/metrics-test.db")
	setMetricFlag(t, "metric-resolution", "45s")
	setMetricFlag(t, "metric-duration", "12m")
	setMetricFlag(t, "namespaces", "team-a,team-b")
	setMetricFlag(t, "v", "3")

	if KubeconfigPath() != "/tmp/metrics-kubeconfig" || DBFile() != "/tmp/metrics-test.db" {
		t.Errorf("paths = %q/%q", KubeconfigPath(), DBFile())
	}
	if MetricResolution() != 45*time.Second || MetricDuration() != 12*time.Minute {
		t.Errorf("durations = %s/%s", MetricResolution(), MetricDuration())
	}
	if got := MetricNamespaces(); len(got) != 2 || got[0] != "team-a" || got[1] != "team-b" {
		t.Errorf("MetricNamespaces() = %v", got)
	}
	if APILogLevel() != LogLevelExtended {
		t.Errorf("APILogLevel() = %v, want %v", APILogLevel(), LogLevelExtended)
	}
}

func TestAPILogLevelFallsBackWhenFlagValueCannotBeParsed(t *testing.T) {
	flag := pflag.Lookup("v")
	previous := flag.Value
	flag.Value = invalidMetricFlagValue("not-a-number")
	t.Cleanup(func() { flag.Value = previous })
	if got := APILogLevel(); got != LogLevelDefault {
		t.Fatalf("APILogLevel(invalid) = %v, want default", got)
	}
}

func TestAPILogLevelUsesDefaultWhenFlagIsMissing(t *testing.T) {
	original := pflag.CommandLine
	t.Cleanup(func() { pflag.CommandLine = original })
	pflag.CommandLine = pflag.NewFlagSet("missing-log-level", pflag.ContinueOnError)
	if got := APILogLevel(); got != LogLevelDefault {
		t.Fatalf("APILogLevel() without v flag = %v, want default %v", got, LogLevelDefault)
	}
}

type invalidMetricFlagValue string

func (in invalidMetricFlagValue) String() string { return string(in) }
func (invalidMetricFlagValue) Set(string) error  { return nil }
func (invalidMetricFlagValue) Type() string      { return "string" }

func setMetricFlag(t *testing.T, name, value string) {
	t.Helper()
	flag := pflag.Lookup(name)
	if flag == nil {
		t.Fatalf("flag %q is not registered", name)
	}
	previous := flag.Value.String()
	t.Cleanup(func() { _ = flag.Value.Set(previous) })
	if err := flag.Value.Set(value); err != nil {
		t.Fatalf("set flag %s: %v", name, err)
	}
}
