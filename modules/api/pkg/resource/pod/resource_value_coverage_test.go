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

package pod

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
)

func TestExtractContainerResourceValue(t *testing.T) {
	container := &corev1.Container{Resources: corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1500m"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		},
	}}

	tests := []struct {
		name     string
		resource string
		divisor  string
		want     string
		wantErr  bool
	}{
		{name: "limits cpu default divisor", resource: "limits.cpu", want: "2"},
		{name: "limits cpu explicit divisor", resource: "limits.cpu", divisor: "1000m", want: "2"},
		{name: "limits memory default divisor", resource: "limits.memory", want: "2147483648"},
		{name: "requests cpu default divisor", resource: "requests.cpu", want: "1"},
		{name: "requests memory explicit divisor", resource: "requests.memory", divisor: "1Gi", want: "1"},
		{name: "unsupported resource", resource: "requests.storage", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fieldSelector := &corev1.ResourceFieldSelector{Resource: test.resource}
			if test.divisor != "" {
				fieldSelector.Divisor = resource.MustParse(test.divisor)
			}
			got, err := extractContainerResourceValue(fieldSelector, container)
			if (err != nil) != test.wantErr {
				t.Fatalf("extractContainerResourceValue() error = %v, wantErr %v", err, test.wantErr)
			}
			if got != test.want {
				t.Errorf("extractContainerResourceValue() = %q, want %q", got, test.want)
			}
		})
	}
}
