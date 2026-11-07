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

package namespace

import (
	"errors"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	"k8s.io/dashboard/api/pkg/resource/common"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestNamespaceCellPropertiesAndConversions(t *testing.T) {
	timestamp := metav1.NewTime(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	namespaces := []corev1.Namespace{{ObjectMeta: metav1.ObjectMeta{Name: "default", CreationTimestamp: timestamp}}}
	cells := toCells(namespaces)
	cell := cells[0].(NamespaceCell)
	if got := cell.GetProperty(dataselect.NameProperty); got != dataselect.StdComparableString("default") {
		t.Errorf("name property = %v", got)
	}
	if got := cell.GetProperty(dataselect.CreationTimestampProperty); got != dataselect.StdComparableTime(timestamp.Time) {
		t.Errorf("creation property = %v", got)
	}
	if got := cell.GetProperty(dataselect.NamespaceProperty); got != dataselect.StdComparableString("") {
		t.Errorf("namespace property = %v", got)
	}
	if got := cell.GetProperty(dataselect.PropertyName("unsupported")); got != nil {
		t.Errorf("unsupported property = %v; want nil", got)
	}
	if got := fromCells(cells); !reflect.DeepEqual(got, namespaces) {
		t.Errorf("fromCells() = %#v; want %#v", got, namespaces)
	}
}

func TestCreateNamespaceCreatesRequestedName(t *testing.T) {
	client := fake.NewSimpleClientset()
	if err := CreateNamespace(&NamespaceSpec{Name: "new-namespace"}, client); err != nil {
		t.Fatalf("CreateNamespace() error = %v", err)
	}
	created, err := client.CoreV1().Namespaces().Get(t.Context(), "new-namespace", metav1.GetOptions{})
	if err != nil || created.Name != "new-namespace" {
		t.Fatalf("created namespace = %#v, %v", created, err)
	}
	client.PrependReactor("create", "namespaces", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("create denied")
	})
	if err := CreateNamespace(&NamespaceSpec{Name: "denied"}, client); err == nil {
		t.Error("CreateNamespace() error = nil; want create failure")
	}
}

func TestGetNamespaceDetailIncludesQuotaAndLimitRange(t *testing.T) {
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}}
	quota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Name: "compute", Namespace: "default"},
		Status: corev1.ResourceQuotaStatus{
			Hard: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("4")},
			Used: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
		},
	}
	limits := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{Name: "container-limits", Namespace: "default"},
		Spec: corev1.LimitRangeSpec{Limits: []corev1.LimitRangeItem{{
			Type: corev1.LimitTypeContainer,
			Max:  corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
		}}},
	}
	client := fake.NewSimpleClientset(namespace, quota, limits)
	got, err := GetNamespaceDetail(client, namespace.Name)
	if err != nil {
		t.Fatalf("GetNamespaceDetail() error = %v", err)
	}
	if got.ObjectMeta.Name != namespace.Name || got.Phase != corev1.NamespaceActive {
		t.Errorf("namespace details = %#v", got.Namespace)
	}
	if len(got.ResourceQuotaList.Items) != 1 || got.ResourceQuotaList.ListMeta.TotalItems != 1 {
		t.Errorf("resource quotas = %#v", got.ResourceQuotaList)
	}
	if len(got.ResourceLimits) != 1 || got.ResourceLimits[0].ResourceName != string(corev1.ResourceMemory) || got.ResourceLimits[0].Max != "1Gi" {
		t.Errorf("resource limits = %#v", got.ResourceLimits)
	}
}

func TestNamespaceDetailPropagatesMissingNamespaceAndLimitRangeErrors(t *testing.T) {
	client := fake.NewSimpleClientset()
	if _, err := GetNamespaceDetail(client, "missing"); err == nil {
		t.Error("GetNamespaceDetail(missing namespace) error = nil")
	}

	client = fake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
	client.PrependReactor("list", "limitranges", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("limit range service unavailable")
	})
	if _, err := GetNamespaceDetail(client, "default"); err == nil {
		t.Error("GetNamespaceDetail(limit range error) error = nil")
	}
}

func TestGetNamespaceListFromChannelsReturnsAndPropagatesErrors(t *testing.T) {
	channels := &common.ResourceChannels{NamespaceList: common.NamespaceListChannel{
		List: make(chan *corev1.NamespaceList, 1), Error: make(chan error, 1),
	}}
	channels.NamespaceList.List <- &corev1.NamespaceList{Items: []corev1.Namespace{{ObjectMeta: metav1.ObjectMeta{Name: "one"}}}}
	channels.NamespaceList.Error <- nil
	got, err := GetNamespaceListFromChannels(channels, dataselect.DefaultDataSelect)
	if err != nil || len(got.Namespaces) != 1 || got.Namespaces[0].ObjectMeta.Name != "one" {
		t.Fatalf("GetNamespaceListFromChannels() = %#v, %v", got, err)
	}

	channels.NamespaceList.List <- &corev1.NamespaceList{}
	channels.NamespaceList.Error <- errors.New("namespace list failed")
	if _, err := GetNamespaceListFromChannels(channels, dataselect.DefaultDataSelect); err == nil {
		t.Error("GetNamespaceListFromChannels(error) error = nil")
	}
}

func TestGetNamespaceListUsesClient(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}})
	got, err := GetNamespaceList(client, dataselect.NoDataSelect)
	if err != nil || got.ListMeta.TotalItems != 1 || got.Namespaces[0].ObjectMeta.Name != "default" || got.Namespaces[0].Phase != corev1.NamespaceActive {
		t.Fatalf("GetNamespaceList() = %#v, %v", got, err)
	}
	client.PrependReactor("list", "namespaces", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("list failed")
	})
	if _, err := GetNamespaceList(client, dataselect.NoDataSelect); err == nil || err.Error() != "list failed" {
		t.Fatalf("GetNamespaceList() error = %v; want list failed", err)
	}
}
