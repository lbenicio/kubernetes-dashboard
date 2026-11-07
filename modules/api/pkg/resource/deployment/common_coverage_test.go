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

package deployment

import (
	"reflect"
	"testing"
	"time"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/dashboard/api/pkg/resource/common"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
	commontypes "k8s.io/dashboard/types"
)

func TestDeploymentCellPropertiesSelectorAndConversions(t *testing.T) {
	timestamp := metav1.NewTime(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	deployment := apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", UID: types.UID("deployment-uid"), CreationTimestamp: timestamp},
		Spec:       apps.DeploymentSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}}},
	}
	cell := DeploymentCell(deployment)
	for _, tc := range []struct {
		property dataselect.PropertyName
		want     dataselect.ComparableValue
	}{
		{dataselect.NameProperty, dataselect.StdComparableString("web")},
		{dataselect.NamespaceProperty, dataselect.StdComparableString("default")},
		{dataselect.CreationTimestampProperty, dataselect.StdComparableTime(timestamp.Time)},
	} {
		if got := cell.GetProperty(tc.property); got != tc.want {
			t.Errorf("GetProperty(%s) = %v; want %v", tc.property, got, tc.want)
		}
	}
	if got := cell.GetProperty(dataselect.PropertyName("unsupported")); got != nil {
		t.Errorf("unsupported property = %v; want nil", got)
	}
	selector := cell.GetResourceSelector()
	if selector.Namespace != "default" || selector.ResourceType != commontypes.ResourceKindDeployment || selector.ResourceName != "web" || selector.UID != "deployment-uid" || !reflect.DeepEqual(selector.Selector, map[string]string{"app": "web"}) {
		t.Errorf("resource selector = %#v", selector)
	}
	cells := toCells([]apps.Deployment{deployment})
	if got := fromCells(cells); !reflect.DeepEqual(got, []apps.Deployment{deployment}) {
		t.Errorf("fromCells() = %#v", got)
	}
}

func TestDeploymentGetStatusClassifiesPodsAndWarnings(t *testing.T) {
	if got := getStatus(nil, nil, nil, nil); got != (common.ResourceStatus{}) {
		t.Fatalf("getStatus(nil) = %#v; want empty status", got)
	}

	deployments := &apps.DeploymentList{Items: []apps.Deployment{
		{ObjectMeta: metav1.ObjectMeta{Name: "pending", UID: "pending-deployment"}, Status: apps.DeploymentStatus{Replicas: 1}},
		{ObjectMeta: metav1.ObjectMeta{Name: "failed", UID: "failed-deployment"}, Status: apps.DeploymentStatus{Replicas: 1}},
		{ObjectMeta: metav1.ObjectMeta{Name: "running", UID: "running-deployment"}, Status: apps.DeploymentStatus{Replicas: 1}},
	}}
	replicaSets := make([]apps.ReplicaSet, 0, 3)
	pods := make([]corev1.Pod, 0, 3)
	for i, name := range []string{"pending", "failed", "running"} {
		deploymentUID := types.UID(name + "-deployment")
		replicaSetUID := types.UID(name + "-replicaset")
		controlled := true
		replicaSets = append(replicaSets, apps.ReplicaSet{ObjectMeta: metav1.ObjectMeta{
			Name: name, UID: replicaSetUID,
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: name, UID: deploymentUID, Controller: &controlled}},
		}})
		phase := corev1.PodRunning
		if i == 0 {
			phase = corev1.PodPending
		}
		status := corev1.PodStatus{Phase: phase}
		if i == 1 {
			status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse}}
		}
		pods = append(pods, corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Name: name + "-pod", UID: types.UID(name + "-pod"),
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: name, UID: replicaSetUID, Controller: &controlled}},
		}, Status: status})
	}
	events := []corev1.Event{{Type: corev1.EventTypeWarning, Reason: "Failed", Message: "image pull failed", InvolvedObject: corev1.ObjectReference{UID: "failed-pod"}}}
	got := getStatus(deployments, replicaSets, pods, events)
	if got.Pending != 1 || got.Failed != 1 || got.Running != 1 {
		t.Errorf("getStatus() = %#v; want one pending, failed, and running deployment", got)
	}
}

func TestGetConditionsMapsDeploymentConditions(t *testing.T) {
	transition := metav1.NewTime(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	updated := metav1.NewTime(time.Date(2026, 11, 7, 11, 0, 0, 0, time.UTC))
	conditions := getConditions([]apps.DeploymentCondition{{
		Type: apps.DeploymentAvailable, Status: corev1.ConditionTrue,
		Reason: "MinimumReplicasAvailable", Message: "ready",
		LastTransitionTime: transition, LastUpdateTime: updated,
	}})
	if len(conditions) != 1 || conditions[0].Type != string(apps.DeploymentAvailable) || conditions[0].Status != corev1.ConditionTrue || conditions[0].Reason != "MinimumReplicasAvailable" || conditions[0].Message != "ready" || !conditions[0].LastTransitionTime.Equal(&transition) || !conditions[0].LastProbeTime.Equal(&updated) {
		t.Errorf("getConditions() = %#v", conditions)
	}
}
