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
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8sClient "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"k8s.io/dashboard/api/pkg/args"
	"k8s.io/dashboard/api/pkg/resource/common"
	"k8s.io/dashboard/types"
)

type Endpoint struct {
	ObjectMeta types.ObjectMeta `json:"objectMeta"`
	TypeMeta   types.TypeMeta   `json:"typeMeta"`

	// Hostname, either as a domain name or IP address.
	Host string `json:"host"`

	// Name of the node the endpoint is located
	NodeName *string `json:"nodeName"`

	// Status of the endpoint
	Ready bool `json:"ready"`

	// Array of endpoint ports
	Ports []v1.EndpointPort `json:"ports"`
}

// GetServiceEndpoints gets list of endpoints targeted by given label selector in given namespace.
func GetServiceEndpoints(client k8sClient.Interface, namespace, name string) (*EndpointList, error) {
	endpointList := &EndpointList{
		Endpoints: make([]Endpoint, 0),
		ListMeta:  types.ListMeta{TotalItems: 0},
	}

	serviceEndpoints, err := GetEndpoints(client, namespace, name)
	if err != nil {
		return endpointList, err
	}

	endpointList = toEndpointList(serviceEndpoints)
	klog.V(args.LogLevelVerbose).Infof("Found %d endpoints related to %s service in %s namespace", len(endpointList.Endpoints), name, namespace)
	return endpointList, nil
}

// GetEndpoints gets EndpointSlices associated to a service with the given name.
func GetEndpoints(client k8sClient.Interface, namespace, name string) ([]discoveryv1.EndpointSlice, error) {
	channels := &common.ResourceChannels{
		EndpointSliceList: common.GetEndpointSliceListChannelWithOptions(client,
			common.NewSameNamespaceQuery(namespace),
			metaV1.ListOptions{
				LabelSelector: labels.Set{discoveryv1.LabelServiceName: name}.AsSelector().String(),
			},
			1),
	}

	endpointSliceList := <-channels.EndpointSliceList.List
	if err := <-channels.EndpointSliceList.Error; err != nil {
		return nil, err
	}

	return endpointSliceList.Items, nil
}

// toEndpoint converts an EndpointSlice address to the Dashboard endpoint model.
func toEndpoint(address string, nodeName *string, ports []v1.EndpointPort, ready bool) *Endpoint {
	return &Endpoint{
		TypeMeta: types.NewTypeMeta(types.ResourceKindEndpoint),
		Host:     address,
		Ports:    ports,
		Ready:    ready,
		NodeName: nodeName,
	}
}
