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

package integration_test

import (
	"testing"

	"k8s.io/dashboard/api/pkg/integration"
	"k8s.io/dashboard/api/pkg/integration/api"
	"k8s.io/dashboard/errors"
)

func TestNewIntegrationManager(t *testing.T) {
	iManager := integration.NewIntegrationManager()
	if iManager == nil {
		t.Error("Failed to create integration manager.")
	}
}

func TestIntegrationManager_GetState(t *testing.T) {
	cases := []struct {
		info          string
		apiServerHost string
		sidecarHost   string
		expected      *api.IntegrationState
		expectedErr   error
	}{
		{
			"Server provided and using external sidecar",
			"http://127.0.0.1:8080", "http://127.0.0.1:8081", &api.IntegrationState{
				Connected: false,
				Error:     errors.NewInvalid("Get http://127.0.0.1:8081/healthz: dial tcp 127.0.0.1:8081: connect: connection refused"),
			}, nil,
		},
	}

	for _, c := range cases {
		iManager := integration.NewIntegrationManager()
		iManager.Metric().ConfigureSidecar(c.sidecarHost)

		state, err := iManager.GetState(api.SidecarIntegrationID)
		if (err == nil) != (c.expectedErr == nil) {
			t.Errorf("Test Case: %s. Expected error to be: %v, but got %v.",
				c.info, c.expectedErr, err)
		}

		if state == nil {
			t.Fatalf("Test Case: %s. Got a nil integration state.", c.info)
		}
		if state.Connected != c.expected.Connected {
			t.Errorf("Test Case: %s. Could not connect to API server.",
				c.info)
		}
		if (state.Error == nil) != (c.expected.Error == nil) {
			t.Errorf("Test Case: %s. Expected state error presence %t, got %v.",
				c.info, c.expected.Error != nil, state.Error)
		}
	}
}

func TestIntegrationManager_Metric(t *testing.T) {
	metricManager := integration.NewIntegrationManager().Metric()
	if metricManager == nil {
		t.Error("failed to get metric manager.")
	}
}

func TestIntegrationManager_GetStateRejectsUnknownIntegration(t *testing.T) {
	iManager := integration.NewIntegrationManager()
	if state, err := iManager.GetState(api.IntegrationID("unknown")); err == nil || state != nil {
		t.Fatalf("GetState(unknown) = %v, %v, want nil state and error", state, err)
	}
}
