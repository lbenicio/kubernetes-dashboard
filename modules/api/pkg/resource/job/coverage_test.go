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

package job

import (
	"testing"
	"time"

	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	"k8s.io/dashboard/api/pkg/resource/common"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
	dashboardtypes "k8s.io/dashboard/types"
)

func TestJobCellPropertiesAndSelector(t *testing.T) {
	created := metav1.NewTime(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	job := batch.Job{ObjectMeta: metav1.ObjectMeta{Name: "task", Namespace: "default", UID: types.UID("uid"), CreationTimestamp: created}}
	cell := JobCell(job)
	for _, tc := range []struct {
		property dataselect.PropertyName
		want     dataselect.ComparableValue
	}{
		{dataselect.NameProperty, dataselect.StdComparableString("task")},
		{dataselect.NamespaceProperty, dataselect.StdComparableString("default")},
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
	if selector.Namespace != "default" || selector.ResourceType != dashboardtypes.ResourceKindJob || selector.ResourceName != "task" || selector.UID != "uid" {
		t.Errorf("GetResourceSelector() = %#v", selector)
	}
	if got := FromCells(ToCells([]batch.Job{job})); len(got) != 1 || got[0].Name != job.Name {
		t.Errorf("cell conversion = %#v", got)
	}
}

func TestClientBackedJobListPodsAndDetail(t *testing.T) {
	completions := int32(1)
	job := &batch.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "task", Namespace: "default", UID: types.UID("job-uid")},
		Spec:       batch.JobSpec{Completions: &completions, Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"job": "task"}}, Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"job": "task"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "task", Image: "busybox:1"}}}}},
		Status:     batch.JobStatus{Active: 1},
	}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "task-pod", Namespace: "default", UID: types.UID("pod-uid"), Labels: map[string]string{"job": "task"}}, Status: corev1.PodStatus{Phase: corev1.PodRunning}}
	client := fake.NewSimpleClientset(job, pod)
	query := common.NewSameNamespaceQuery("default")
	list, err := GetJobList(client, query, dataselect.NoDataSelect, nil)
	if err != nil || list.ListMeta.TotalItems != 1 || len(list.Jobs) != 1 || list.Jobs[0].ObjectMeta.Name != "task" {
		t.Fatalf("GetJobList() = %#v, %v", list, err)
	}
	pods, err := GetJobPods(client, nil, dataselect.NoDataSelect, "default", "task")
	if err != nil || len(pods.Pods) != 1 || pods.Pods[0].ObjectMeta.Name != "task-pod" {
		t.Fatalf("GetJobPods() = %#v, %v", pods, err)
	}
	detail, err := GetJobDetail(client, "default", "task")
	if err != nil || detail.ObjectMeta.Name != "task" || detail.Completions == nil || *detail.Completions != 1 {
		t.Fatalf("GetJobDetail() = %#v, %v", detail, err)
	}
	if _, err := GetJobList(client, common.NewNamespaceQuery(nil), dataselect.NoDataSelect, nil); err != nil {
		t.Fatalf("GetJobList(all namespaces): %v", err)
	}
	if _, err := GetJobPods(client, nil, dataselect.NoDataSelect, "default", "missing"); err == nil {
		t.Fatal("GetJobPods() succeeded for missing job")
	}
}
