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

package endpoint

import (
	"reflect"
	"sort"
	"testing"

	v1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"k8s.io/dashboard/types"
)

func TestGetServiceEndpoints(t *testing.T) {
	ready := true
	notReady := false
	portNumber := int32(8080)
	portName := "http"
	protocol := v1.ProtocolTCP
	nodeName := "node-1"

	makeSlice := func(name, serviceName, address string, endpointReady *bool) *discoveryv1.EndpointSlice {
		return &discoveryv1.EndpointSlice{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      name,
				Namespace: "default",
				Labels:    map[string]string{discoveryv1.LabelServiceName: serviceName},
			},
			Ports: []discoveryv1.EndpointPort{{
				Name:     &portName,
				Port:     &portNumber,
				Protocol: &protocol,
			}},
			Endpoints: []discoveryv1.Endpoint{{
				Addresses: []string{address},
				NodeName:  &nodeName,
				Conditions: discoveryv1.EndpointConditions{
					Ready: endpointReady,
				},
			}},
		}
	}

	client := fake.NewSimpleClientset(
		makeSlice("service-ready", "service", "10.0.0.1", &ready),
		makeSlice("service-not-ready", "service", "10.0.0.2", &notReady),
		makeSlice("other-service", "other", "10.0.0.3", &ready),
	)

	actual, err := GetServiceEndpoints(client, "default", "service")
	if err != nil {
		t.Fatalf("GetServiceEndpoints() error = %v", err)
	}
	sort.Slice(actual.Endpoints, func(i, j int) bool {
		return actual.Endpoints[i].Host < actual.Endpoints[j].Host
	})

	want := &EndpointList{
		ListMeta: types.ListMeta{TotalItems: 2},
		Endpoints: []Endpoint{
			{
				TypeMeta: types.TypeMeta{Kind: types.ResourceKindEndpoint},
				Host:     "10.0.0.1",
				NodeName: &nodeName,
				Ready:    true,
				Ports: []v1.EndpointPort{{
					Name:     portName,
					Port:     portNumber,
					Protocol: protocol,
				}},
			},
			{
				TypeMeta: types.TypeMeta{Kind: types.ResourceKindEndpoint},
				Host:     "10.0.0.2",
				NodeName: &nodeName,
				Ready:    false,
				Ports: []v1.EndpointPort{{
					Name:     portName,
					Port:     portNumber,
					Protocol: protocol,
				}},
			},
		},
	}
	if !reflect.DeepEqual(actual, want) {
		t.Errorf("GetServiceEndpoints() = %#v, want %#v", actual, want)
	}

	actions := client.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected one list action, got %d", len(actions))
	}
	listAction, ok := actions[0].(k8stesting.ListAction)
	if !ok {
		t.Fatalf("expected a list action, got %T", actions[0])
	}
	if got, want := listAction.GetListRestrictions().Labels.String(), discoveryv1.LabelServiceName+"=service"; got != want {
		t.Errorf("list label selector = %q, want %q", got, want)
	}
}

func TestToEndpointListTreatsUnknownReadinessAsReady(t *testing.T) {
	actual := toEndpointList([]discoveryv1.EndpointSlice{{
		Endpoints: []discoveryv1.Endpoint{{
			Addresses: []string{"10.0.0.1", "10.0.0.2"},
		}},
	}})

	if len(actual.Endpoints) != 2 {
		t.Fatalf("endpoint count = %d, want 2", len(actual.Endpoints))
	}
	for _, endpoint := range actual.Endpoints {
		if !endpoint.Ready {
			t.Errorf("endpoint %q should be ready when readiness is unknown", endpoint.Host)
		}
	}
}
