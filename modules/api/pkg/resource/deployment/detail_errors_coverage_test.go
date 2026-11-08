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

package deployment

import (
	"errors"
	"testing"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestGetDeploymentDetailReturnsCriticalKubernetesFailures(t *testing.T) {
	wantErr := errors.New("resource request failed")
	validDeployment := func() *apps.Deployment {
		return &apps.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
			Spec: apps.DeploymentSpec{
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
			},
		}
	}
	for _, test := range []struct {
		name   string
		client func() *fake.Clientset
	}{
		{
			name: "deployment get",
			client: func() *fake.Clientset {
				client := fake.NewSimpleClientset()
				client.PrependReactor("get", "deployments", func(ktesting.Action) (bool, runtime.Object, error) { return true, nil, wantErr })
				return client
			},
		},
		{
			name: "invalid label selector",
			client: func() *fake.Clientset {
				deployment := validDeployment()
				deployment.Spec.Selector = &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "app", Operator: "INVALID"}}}
				return fake.NewSimpleClientset(deployment)
			},
		},
		{
			name: "replica set list",
			client: func() *fake.Clientset {
				client := fake.NewSimpleClientset(validDeployment())
				client.PrependReactor("list", "replicasets", func(ktesting.Action) (bool, runtime.Object, error) { return true, &apps.ReplicaSetList{}, wantErr })
				return client
			},
		},
		{
			name: "pod list",
			client: func() *fake.Clientset {
				client := fake.NewSimpleClientset(validDeployment())
				client.PrependReactor("list", "pods", func(ktesting.Action) (bool, runtime.Object, error) { return true, &corev1.PodList{}, wantErr })
				return client
			},
		},
		{
			name: "event list",
			client: func() *fake.Clientset {
				client := fake.NewSimpleClientset(validDeployment())
				client.PrependReactor("list", "events", func(ktesting.Action) (bool, runtime.Object, error) { return true, &corev1.EventList{}, wantErr })
				return client
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := GetDeploymentDetail(test.client(), "default", "web"); err == nil {
				t.Fatal("GetDeploymentDetail() succeeded after a Kubernetes failure")
			}
		})
	}
}
