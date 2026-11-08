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

package daemonset

import (
	"errors"
	"testing"

	apps "k8s.io/api/apps/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"

	"k8s.io/dashboard/api/pkg/resource/common"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestDaemonSetDetailReportsRelatedResourceFailures(t *testing.T) {
	daemonSet := &apps.DaemonSet{
		ObjectMeta: meta.ObjectMeta{Name: "agents", Namespace: "default", Labels: map[string]string{"app": "agent"}},
		Spec:       apps.DaemonSetSpec{Selector: &meta.LabelSelector{MatchLabels: map[string]string{"app": "agent"}}},
	}
	tests := []struct {
		name string
		verb string
		kind string
	}{
		{name: "daemonset get", verb: "get", kind: "daemonsets"},
		{name: "event list", verb: "list", kind: "events"},
		{name: "pod list", verb: "list", kind: "pods"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := fake.NewSimpleClientset(daemonSet)
			client.PrependReactor(test.verb, test.kind, func(ktesting.Action) (bool, runtime.Object, error) {
				return true, nil, errors.New("simulated Kubernetes failure")
			})
			if _, err := GetDaemonSetDetail(client, nil, "default", "agents"); err == nil {
				t.Fatal("GetDaemonSetDetail() succeeded despite an injected API failure")
			}
		})
	}
}

func TestDaemonSetServiceHelpersReportLookupAndListFailures(t *testing.T) {
	daemonSet := &apps.DaemonSet{
		ObjectMeta: meta.ObjectMeta{Name: "agents", Namespace: "default", Labels: map[string]string{"app": "agent"}},
		Spec:       apps.DaemonSetSpec{Selector: &meta.LabelSelector{MatchLabels: map[string]string{"app": "agent"}}},
	}
	for _, test := range []struct {
		name     string
		verb     string
		resource string
		seed     bool
	}{
		{name: "daemonset lookup", verb: "get", resource: "daemonsets", seed: true},
		{name: "services list", verb: "list", resource: "services", seed: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var client *fake.Clientset
			if test.seed {
				client = fake.NewSimpleClientset(daemonSet)
			} else {
				client = fake.NewSimpleClientset()
			}
			client.PrependReactor(test.verb, test.resource, func(ktesting.Action) (bool, runtime.Object, error) {
				return true, nil, errors.New("simulated Kubernetes failure")
			})
			if _, err := GetDaemonSetServices(client, dataselect.NoDataSelect, "default", "agents"); err == nil {
				t.Fatal("GetDaemonSetServices() returned no error after injected API failure")
			}
		})
	}

	for _, test := range []struct {
		name     string
		resource string
		seed     []runtime.Object
	}{
		{name: "daemonset list failure", resource: "daemonsets"},
		{name: "service list failure", resource: "services", seed: []runtime.Object{daemonSet}},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := fake.NewSimpleClientset(test.seed...)
			client.PrependReactor("list", test.resource, func(ktesting.Action) (bool, runtime.Object, error) {
				return true, nil, errors.New("simulated Kubernetes failure")
			})
			if _, err := GetServicesForDSDeletion(client, labels.Set{"app": "agent"}.AsSelector(), "default"); err == nil {
				t.Fatal("GetServicesForDSDeletion() returned no error after injected API failure")
			}
		})
	}

	// With zero matching daemon sets there is no unique target and no service query is needed.
	client := fake.NewSimpleClientset()
	services, err := GetServicesForDSDeletion(client, labels.Set{"app": "agent"}.AsSelector(), "default")
	if err != nil || len(services) != 0 {
		t.Fatalf("GetServicesForDSDeletion(empty) = %v, %v; want empty result", services, err)
	}
}

func TestDaemonSetListReportsFailuresFromEveryChannel(t *testing.T) {
	for _, resource := range []string{"daemonsets", "pods", "events"} {
		t.Run(resource, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			client.PrependReactor("list", resource, func(ktesting.Action) (bool, runtime.Object, error) {
				return true, nil, errors.New("simulated Kubernetes failure")
			})
			if _, err := GetDaemonSetList(client, common.NewNamespaceQuery(nil), dataselect.NoDataSelect, nil); err == nil {
				t.Fatalf("GetDaemonSetList() returned no error for failed %s list", resource)
			}
		})
	}
}

func TestGetDaemonSetPodsReturnsEventListFailure(t *testing.T) {
	daemonSet := &apps.DaemonSet{
		ObjectMeta: meta.ObjectMeta{Name: "agents", Namespace: "default"},
		Spec:       apps.DaemonSetSpec{Selector: &meta.LabelSelector{MatchLabels: map[string]string{"app": "agent"}}},
	}
	client := fake.NewSimpleClientset(daemonSet)
	client.PrependReactor("list", "events", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("event list failed")
	})
	if _, err := GetDaemonSetPods(client, nil, dataselect.NoDataSelect, daemonSet.Name, daemonSet.Namespace); err == nil {
		t.Fatal("GetDaemonSetPods() error = nil; want event list failure")
	}
}
