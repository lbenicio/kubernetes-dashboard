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
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package deployment

import (
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestDeploymentPodAndReplicaSetViewsPropagateReadErrors(t *testing.T) {
	query := dataselect.NoDataSelect
	for _, test := range []struct {
		name     string
		resource string
		call     func(*fake.Clientset) error
	}{
		{
			name: "deployment pods reject pod-list error", resource: "pods",
			call: func(client *fake.Clientset) error {
				_, err := GetDeploymentPods(client, nil, query, "default", "web")
				return err
			},
		},
		{
			name: "deployment pods reject replica-set-list error", resource: "replicasets",
			call: func(client *fake.Clientset) error {
				_, err := GetDeploymentPods(client, nil, query, "default", "web")
				return err
			},
		},
		{
			name: "deployment pods reject event-list error", resource: "events",
			call: func(client *fake.Clientset) error {
				_, err := GetDeploymentPods(client, nil, query, "default", "web")
				return err
			},
		},
		{
			name: "new replica set rejects replica-set-list error", resource: "replicasets",
			call: func(client *fake.Clientset) error {
				_, err := GetDeploymentNewReplicaSet(client, query, "default", "web")
				return err
			},
		},
		{
			name: "new replica set rejects pod-list error", resource: "pods",
			call: func(client *fake.Clientset) error {
				_, err := GetDeploymentNewReplicaSet(client, query, "default", "web")
				return err
			},
		},
		{
			name: "new replica set rejects event-list error", resource: "events",
			call: func(client *fake.Clientset) error {
				_, err := GetDeploymentNewReplicaSet(client, query, "default", "web")
				return err
			},
		},
		{
			name: "old replica sets reject replica-set-list error", resource: "replicasets",
			call: func(client *fake.Clientset) error {
				_, err := GetDeploymentOldReplicaSets(client, query, "default", "web")
				return err
			},
		},
		{
			name: "old replica sets reject pod-list error", resource: "pods",
			call: func(client *fake.Clientset) error {
				_, err := GetDeploymentOldReplicaSets(client, query, "default", "web")
				return err
			},
		},
		{
			name: "old replica sets reject event-list error", resource: "events",
			call: func(client *fake.Clientset) error {
				_, err := GetDeploymentOldReplicaSets(client, query, "default", "web")
				return err
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			client, _ := deploymentClientFixture()
			client.PrependReactor("list", test.resource, func(clienttesting.Action) (bool, runtime.Object, error) {
				return true, nil, errors.New("list rejected")
			})
			if err := test.call(client); err == nil {
				t.Fatal("call error = nil; want list error")
			}
		})
	}
}

func TestDeploymentPodAndReplicaSetViewsRejectMissingDeployments(t *testing.T) {
	query := dataselect.NoDataSelect
	client := fake.NewSimpleClientset()
	if _, err := GetDeploymentPods(client, nil, query, "default", "missing"); err == nil {
		t.Error("GetDeploymentPods(missing deployment) error = nil")
	}
	if _, err := GetDeploymentNewReplicaSet(client, query, "default", "missing"); err == nil {
		t.Error("GetDeploymentNewReplicaSet(missing deployment) error = nil")
	}
	if _, err := GetDeploymentOldReplicaSets(client, query, "default", "missing"); err == nil {
		t.Error("GetDeploymentOldReplicaSets(missing deployment) error = nil")
	}

	invalidSelector := deploymentClientFixtureWithSelector(&metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "app", Operator: "INVALID"}}})
	if _, err := GetDeploymentNewReplicaSet(invalidSelector, query, "default", "web"); err == nil {
		t.Error("GetDeploymentNewReplicaSet(invalid selector) error = nil")
	}
	if _, err := GetDeploymentOldReplicaSets(invalidSelector, query, "default", "web"); err == nil {
		t.Error("GetDeploymentOldReplicaSets(invalid selector) error = nil")
	}
}

func TestGetDeploymentNewReplicaSetReturnsEmptyViewWhenNoneExists(t *testing.T) {
	_, deployment := deploymentClientFixture()
	client := fake.NewSimpleClientset(deployment)
	got, err := GetDeploymentNewReplicaSet(client, dataselect.NoDataSelect, "default", "web")
	if err != nil {
		t.Fatalf("GetDeploymentNewReplicaSet() error = %v", err)
	}
	if got == nil || got.ObjectMeta.Name != "" {
		t.Fatalf("GetDeploymentNewReplicaSet() = %#v; want an empty view when no matching ReplicaSet exists", got)
	}
}

func deploymentClientFixtureWithSelector(selector *metav1.LabelSelector) *fake.Clientset {
	client, deployment := deploymentClientFixture()
	deployment.Spec.Selector = selector
	client = fake.NewSimpleClientset(deployment)
	return client
}
