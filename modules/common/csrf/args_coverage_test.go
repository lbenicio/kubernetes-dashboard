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
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package csrf

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestFatalCSRFConfigurationPaths(t *testing.T) {
	if action := os.Getenv("CSRF_FATAL_HELPER"); action != "" {
		switch action {
		case "invalid-key":
			*argCSRFKey = "not-base64"
			Ensure()
		case "uninitialized-key":
			decodedCSRFKey = ""
			Key()
		default:
			t.Fatalf("unknown helper action %q", action)
		}
		return
	}

	for _, test := range []struct {
		name        string
		wantMessage string
	}{
		{name: "invalid-key", wantMessage: "Could not decode CSRF key"},
		{name: "uninitialized-key", wantMessage: "CSRF key was not properly initialized"},
	} {
		t.Run(test.name, func(t *testing.T) {
			command := exec.Command(os.Args[0], "-test.run=^TestFatalCSRFConfigurationPaths$")
			command.Env = append(os.Environ(), "CSRF_FATAL_HELPER="+test.name)
			output, err := command.CombinedOutput()
			var exitError *exec.ExitError
			if !errors.As(err, &exitError) || exitError.ExitCode() != 255 {
				t.Fatalf("helper exit = %v, want exit code 255; output: %s", err, output)
			}
			if !strings.Contains(string(output), test.wantMessage) {
				t.Errorf("helper output %q does not contain %q", output, test.wantMessage)
			}
		})
	}
}
