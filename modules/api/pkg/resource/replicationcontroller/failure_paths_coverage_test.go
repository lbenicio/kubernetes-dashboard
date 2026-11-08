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

package replicationcontroller

import (
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"

	"k8s.io/dashboard/api/pkg/resource/common"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestReplicationControllerDetailsAndPodsReportRelatedResourceFailures(t *testing.T) {
	controller := &corev1.ReplicationController{
		ObjectMeta: meta.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       corev1.ReplicationControllerSpec{Selector: map[string]string{"app": "web"}},
	}
	tests := []struct {
		name string
		verb string
		kind string
		call func(*fake.Clientset) error
	}{
		{name: "detail get", verb: "get", kind: "replicationcontrollers", call: func(c *fake.Clientset) error {
			_, err := GetReplicationControllerDetail(c, "default", "web")
			return err
		}},
		{name: "detail pod list", verb: "list", kind: "pods", call: func(c *fake.Clientset) error {
			_, err := GetReplicationControllerDetail(c, "default", "web")
			return err
		}},
		{name: "pods controller get", verb: "get", kind: "replicationcontrollers", call: func(c *fake.Clientset) error {
			_, err := GetReplicationControllerPods(c, nil, dataselect.NoDataSelect, "web", "default")
			return err
		}},
		{name: "pods list", verb: "list", kind: "pods", call: func(c *fake.Clientset) error {
			_, err := GetReplicationControllerPods(c, nil, dataselect.NoDataSelect, "web", "default")
			return err
		}},
		{name: "pods events", verb: "list", kind: "events", call: func(c *fake.Clientset) error {
			c.PrependReactor("list", "pods", func(ktesting.Action) (bool, runtime.Object, error) {
				return true, &corev1.PodList{}, nil
			})
			_, err := GetReplicationControllerPods(c, nil, dataselect.NoDataSelect, "web", "default")
			return err
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := fake.NewSimpleClientset(controller)
			client.PrependReactor(test.verb, test.kind, func(ktesting.Action) (bool, runtime.Object, error) {
				return true, nil, errors.New("simulated Kubernetes failure")
			})
			if err := test.call(client); err == nil {
				t.Fatal("operation succeeded despite the injected API failure")
			}
		})
	}
}

func TestReplicationControllerServicesReportLookupAndListFailures(t *testing.T) {
	for _, test := range []struct {
		name     string
		resource string
	}{
		{name: "controller lookup", resource: "replicationcontrollers"},
		{name: "service list", resource: "services"},
	} {
		t.Run(test.name, func(t *testing.T) {
			controller := &corev1.ReplicationController{
				ObjectMeta: meta.ObjectMeta{Name: "web", Namespace: "default"},
				Spec:       corev1.ReplicationControllerSpec{Selector: map[string]string{"app": "web"}},
			}
			client := fake.NewSimpleClientset(controller)
			if test.resource == "replicationcontrollers" {
				client.PrependReactor("get", test.resource, func(ktesting.Action) (bool, runtime.Object, error) { return true, nil, errors.New("failed") })
			} else {
				client.PrependReactor("list", test.resource, func(ktesting.Action) (bool, runtime.Object, error) { return true, nil, errors.New("failed") })
			}
			if _, err := GetReplicationControllerServices(client, dataselect.NoDataSelect, "default", "web"); err == nil {
				t.Fatal("GetReplicationControllerServices() returned no error after injected API failure")
			}
		})
	}
}

func TestUpdateReplicasCountReportsGetAndUpdateFailures(t *testing.T) {
	controller := &corev1.ReplicationController{
		ObjectMeta: meta.ObjectMeta{Name: "web", Namespace: "default"},
	}
	spec := &ReplicationControllerSpec{Replicas: 3}

	t.Run("get failure", func(t *testing.T) {
		client := fake.NewSimpleClientset(controller)
		client.PrependReactor("get", "replicationcontrollers", func(ktesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("controller get failed")
		})
		if err := UpdateReplicasCount(client, "default", "web", spec); err == nil {
			t.Fatal("UpdateReplicasCount() succeeded after the controller get failed")
		}
	})

	t.Run("update failure", func(t *testing.T) {
		client := fake.NewSimpleClientset(controller)
		client.PrependReactor("update", "replicationcontrollers", func(ktesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("controller update failed")
		})
		if err := UpdateReplicasCount(client, "default", "web", spec); err == nil {
			t.Fatal("UpdateReplicasCount() succeeded after the controller update failed")
		}
	})
}

func TestReplicationControllerListReportsFailuresFromEveryChannel(t *testing.T) {
	for _, resource := range []string{"replicationcontrollers", "pods", "events"} {
		t.Run(resource, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			client.PrependReactor("list", resource, func(ktesting.Action) (bool, runtime.Object, error) {
				return true, nil, errors.New("simulated Kubernetes failure")
			})
			if _, err := GetReplicationControllerList(client, common.NewNamespaceQuery(nil), dataselect.NoDataSelect, nil); err == nil {
				t.Fatalf("GetReplicationControllerList() returned no error for failed %s list", resource)
			}
		})
	}
}
