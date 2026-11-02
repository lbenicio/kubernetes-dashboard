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

package endpoint

import (
	v1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"

	"k8s.io/dashboard/types"
)

type EndpointList struct {
	ListMeta types.ListMeta `json:"listMeta"`
	// List of endpoints
	Endpoints []Endpoint `json:"endpoints"`
}

// toEndpointList converts EndpointSlices to the Dashboard endpoint list structure.
func toEndpointList(endpointSlices []discoveryv1.EndpointSlice) *EndpointList {
	endpointList := EndpointList{
		Endpoints: make([]Endpoint, 0),
		ListMeta:  types.ListMeta{},
	}

	for _, endpointSlice := range endpointSlices {
		ports := toEndpointPorts(endpointSlice.Ports)
		for _, endpoint := range endpointSlice.Endpoints {
			ready := endpoint.Conditions.Ready == nil || *endpoint.Conditions.Ready
			for _, address := range endpoint.Addresses {
				endpointList.Endpoints = append(endpointList.Endpoints, *toEndpoint(address, endpoint.NodeName, ports, ready))
			}
		}
	}

	endpointList.ListMeta.TotalItems = len(endpointList.Endpoints)
	return &endpointList
}

func toEndpointPorts(endpointPorts []discoveryv1.EndpointPort) []v1.EndpointPort {
	ports := make([]v1.EndpointPort, 0, len(endpointPorts))
	for _, endpointPort := range endpointPorts {
		port := v1.EndpointPort{Protocol: v1.ProtocolTCP}
		if endpointPort.Name != nil {
			port.Name = *endpointPort.Name
		}
		if endpointPort.Port != nil {
			port.Port = *endpointPort.Port
		}
		if endpointPort.Protocol != nil {
			port.Protocol = *endpointPort.Protocol
		}
		port.AppProtocol = endpointPort.AppProtocol
		ports = append(ports, port)
	}
	return ports
}
