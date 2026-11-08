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

package replicaset

import (
	"errors"
	"testing"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"

	"k8s.io/dashboard/api/pkg/resource/common"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestReplicaSetDetailAndPodsReportRelatedResourceFailures(t *testing.T) {
	selector := &meta.LabelSelector{MatchLabels: map[string]string{"app": "web"}}
	replicaSet := &apps.ReplicaSet{ObjectMeta: meta.ObjectMeta{Name: "web", Namespace: "default"}, Spec: apps.ReplicaSetSpec{Selector: selector}}

	tests := []struct {
		name string
		verb string
		kind string
		call func(*fake.Clientset) error
	}{
		{name: "detail get", verb: "get", kind: "replicasets", call: func(c *fake.Clientset) error {
			_, err := GetReplicaSetDetail(c, nil, "default", "web")
			return err
		}},
		{name: "detail pod list", verb: "list", kind: "pods", call: func(c *fake.Clientset) error {
			_, err := GetReplicaSetDetail(c, nil, "default", "web")
			return err
		}},
		{name: "detail autoscaler list", verb: "list", kind: "horizontalpodautoscalers", call: func(c *fake.Clientset) error {
			c.PrependReactor("list", "pods", func(ktesting.Action) (bool, runtime.Object, error) {
				return true, &corev1.PodList{}, nil
			})
			_, err := GetReplicaSetDetail(c, nil, "default", "web")
			return err
		}},
		{name: "pods replica set get", verb: "get", kind: "replicasets", call: func(c *fake.Clientset) error {
			_, err := GetReplicaSetPods(c, nil, dataselect.NoDataSelect, "web", "default")
			return err
		}},
		{name: "pods list", verb: "list", kind: "pods", call: func(c *fake.Clientset) error {
			_, err := GetReplicaSetPods(c, nil, dataselect.NoDataSelect, "web", "default")
			return err
		}},
		{name: "pods events", verb: "list", kind: "events", call: func(c *fake.Clientset) error {
			c.PrependReactor("list", "pods", func(ktesting.Action) (bool, runtime.Object, error) {
				return true, &corev1.PodList{}, nil
			})
			_, err := GetReplicaSetPods(c, nil, dataselect.NoDataSelect, "web", "default")
			return err
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := fake.NewSimpleClientset(replicaSet)
			client.PrependReactor(test.verb, test.kind, func(ktesting.Action) (bool, runtime.Object, error) {
				return true, nil, errors.New("simulated Kubernetes failure")
			})
			if err := test.call(client); err == nil {
				t.Fatal("operation succeeded despite the injected API failure")
			}
		})
	}
}

func TestReplicaSetServicesReportLookupAndListFailures(t *testing.T) {
	for _, test := range []struct {
		name     string
		resource string
		seed     runtime.Object
	}{
		{name: "replica set lookup", resource: "replicasets"},
		{name: "service list", resource: "services", seed: &apps.ReplicaSet{ObjectMeta: meta.ObjectMeta{Name: "web", Namespace: "default"}, Spec: apps.ReplicaSetSpec{Selector: &meta.LabelSelector{MatchLabels: map[string]string{"app": "web"}}}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			if test.seed != nil {
				client = fake.NewSimpleClientset(test.seed)
			}
			if test.resource == "replicasets" {
				client.PrependReactor("get", "replicasets", func(ktesting.Action) (bool, runtime.Object, error) { return true, nil, errors.New("failed") })
			} else {
				client.PrependReactor("list", "services", func(ktesting.Action) (bool, runtime.Object, error) { return true, nil, errors.New("failed") })
			}
			if _, err := GetReplicaSetServices(client, dataselect.NoDataSelect, "default", "web"); err == nil {
				t.Fatal("GetReplicaSetServices() returned no error after injected API failure")
			}
		})
	}
}

func TestReplicaSetListReportsFailuresFromEveryChannel(t *testing.T) {
	for _, resource := range []string{"replicasets", "pods", "events"} {
		t.Run(resource, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			client.PrependReactor("list", resource, func(ktesting.Action) (bool, runtime.Object, error) {
				return true, nil, errors.New("simulated Kubernetes failure")
			})
			if _, err := GetReplicaSetList(client, common.NewNamespaceQuery(nil), dataselect.NoDataSelect, nil); err == nil {
				t.Fatalf("GetReplicaSetList() returned no error for failed %s list", resource)
			}
		})
	}
}
