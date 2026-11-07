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

package metric

import (
	"reflect"
	"testing"

	integrationapi "k8s.io/dashboard/api/pkg/integration/api"
	"k8s.io/dashboard/api/pkg/integration/metric/api"
	"k8s.io/dashboard/errors"
)

const fakeMetricClientID integrationapi.IntegrationID = "test-id"

type FakeMetricClient struct {
	healthOk bool
}

func (FakeMetricClient) ID() integrationapi.IntegrationID {
	return fakeMetricClientID
}

func (in FakeMetricClient) HealthCheck() error {
	if in.healthOk {
		return nil
	}

	return errors.NewInvalid("test-error")
}

func (in FakeMetricClient) DownloadMetric(selectors []api.ResourceSelector, metricName string,
	cachedResources *api.CachedResources) api.MetricPromises {
	return nil
}

func (in FakeMetricClient) DownloadMetrics(selectors []api.ResourceSelector, metricNames []string,
	cachedResources *api.CachedResources) api.MetricPromises {
	return nil
}

func (in FakeMetricClient) AggregateMetrics(metrics api.MetricPromises, metricName string,
	aggregations api.AggregationModes) api.MetricPromises {
	return nil
}

func areErrorsEqual(err1, err2 error) bool {
	return (err1 != nil && err2 != nil && err1.Error() == err2.Error()) ||
		(err1 == nil && err2 == nil)
}

func TestNewMetricManager(t *testing.T) {
	metricManager := NewMetricManager()
	if metricManager == nil {
		t.Error("Failed to create metric manager.")
	}
}

func TestMetricManager_Client(t *testing.T) {
	cases := []struct {
		client   api.MetricClient
		expected api.MetricClient
	}{
		{&FakeMetricClient{healthOk: false}, nil},
		{&FakeMetricClient{healthOk: true}, &FakeMetricClient{healthOk: true}},
	}

	for _, c := range cases {
		metricManager := NewMetricManager()
		metricManager.AddClient(c.client)
		_ = metricManager.Enable(fakeMetricClientID)
		client := metricManager.Client()

		if !reflect.DeepEqual(client, c.expected) {
			t.Errorf("Failed to get active metric client. Expected: %v, but got %v.",
				c.expected, client)
		}
	}
}

func TestMetricManager_Enable(t *testing.T) {
	cases := []struct {
		client   api.MetricClient
		expected error
	}{
		{&FakeMetricClient{healthOk: false}, errors.NewInvalid("health check failed: test-error")},
		{&FakeMetricClient{healthOk: true}, nil},
	}

	for _, c := range cases {
		metricManager := NewMetricManager()
		metricManager.AddClient(c.client)
		err := metricManager.Enable(fakeMetricClientID)

		if !areErrorsEqual(err, c.expected) {
			t.Errorf("Failed to enable metric client. Expected error to be %v, but "+
				"got %v.", c.expected, err)
		}
	}
}

func TestMetricManager_List(t *testing.T) {
	cases := []struct {
		client          api.MetricClient
		expectedClients int
	}{
		{&FakeMetricClient{healthOk: false}, 1},
		{nil, 0},
	}

	for _, c := range cases {
		metricManager := NewMetricManager()
		metricManager.AddClient(c.client)
		list := metricManager.List()

		if len(list) != c.expectedClients {
			t.Errorf("Expected number of clients to be %v, but got %v.",
				c.expectedClients, len(list))
		}
	}
}

func TestMetricManagerEnableAndConfigureFailures(t *testing.T) {
	manager := NewMetricManager()
	manager.AddClient(nil)
	if len(manager.List()) != 0 {
		t.Fatal("AddClient(nil) should not register a client")
	}
	if err := manager.Enable("missing"); err == nil {
		t.Fatal("Enable() succeeded for an unknown integration")
	}
	if manager.ConfigureSidecar("%") != manager || len(manager.List()) != 0 {
		t.Fatal("ConfigureSidecar() should retain the manager and leave invalid host unregistered")
	}

	healthy := &FakeMetricClient{healthOk: true}
	manager.AddClient(healthy)
	if err := manager.Enable(fakeMetricClientID); err != nil || manager.Client() != healthy {
		t.Fatalf("Enable(healthy) error = %v; active client = %v", err, manager.Client())
	}
	manager.AddClient(&FakeMetricClient{healthOk: false})
	if err := manager.Enable(fakeMetricClientID); err == nil {
		t.Fatal("Enable() succeeded when health check failed")
	}
	if manager.Client() != healthy {
		t.Fatal("failed health check replaced the active client")
	}
}
