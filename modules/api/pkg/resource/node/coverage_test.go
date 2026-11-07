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

package node

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestNodeCellPropertiesAndSelector(t *testing.T) {
	created := metav1.NewTime(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	node := corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-a", UID: types.UID("uid"), CreationTimestamp: created}}
	cell := NodeCell(node)
	for _, tc := range []struct {
		property dataselect.PropertyName
		want     dataselect.ComparableValue
	}{
		{dataselect.NameProperty, dataselect.StdComparableString("worker-a")},
		{dataselect.NamespaceProperty, dataselect.StdComparableString("")},
		{dataselect.CreationTimestampProperty, dataselect.StdComparableTime(created.Time)},
	} {
		if got := cell.GetProperty(tc.property); got != tc.want {
			t.Errorf("GetProperty(%s) = %v; want %v", tc.property, got, tc.want)
		}
	}
	if cell.GetProperty(dataselect.PropertyName("unsupported")) != nil {
		t.Error("unsupported property should return nil")
	}
	selector := cell.GetResourceSelector()
	if selector.ResourceName != "worker-a" || selector.ResourceType != "node" || selector.UID != "uid" {
		t.Errorf("GetResourceSelector() = %#v", selector)
	}
	if got := fromCells(toCells([]corev1.Node{node})); len(got) != 1 || got[0].Name != node.Name {
		t.Errorf("cell conversion = %#v", got)
	}
}

func TestDrainNodeHandlesMissingAndEmptyNode(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-a"}})
	if err := DrainNode(client, "missing", nil); err == nil {
		t.Fatal("DrainNode() succeeded for missing node")
	}
	if err := DrainNode(client, "worker-a", nil); err != nil {
		t.Fatalf("DrainNode() for empty node: %v", err)
	}
	cordoned, err := client.CoreV1().Nodes().Get(t.Context(), "worker-a", metav1.GetOptions{})
	if err != nil || !cordoned.Spec.Unschedulable {
		t.Fatalf("node after drain = %#v, %v; want unschedulable", cordoned, err)
	}
}
