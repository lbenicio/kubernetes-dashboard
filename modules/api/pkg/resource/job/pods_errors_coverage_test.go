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
	"errors"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestGetJobPodsReturnsKubernetesFailures(t *testing.T) {
	selector := &metav1.LabelSelector{MatchLabels: map[string]string{"app": "batch"}}
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "batch", Namespace: "default"}, Spec: batchv1.JobSpec{Selector: selector}}
	for _, test := range []struct {
		name     string
		resource string
	}{
		{name: "job lookup", resource: "jobs"},
		{name: "pod list", resource: "pods"},
		{name: "event list", resource: "events"},
	} {
		t.Run(test.name, func(t *testing.T) {
			wantErr := errors.New(test.name + " failed")
			client := fake.NewSimpleClientset(job)
			if test.resource == "jobs" {
				client = fake.NewSimpleClientset()
			}
			client.PrependReactor("list", test.resource, func(ktesting.Action) (bool, runtime.Object, error) {
				if test.resource == "jobs" {
					return false, nil, nil
				}
				return true, nil, wantErr
			})
			if test.resource == "jobs" {
				client.PrependReactor("get", "jobs", func(ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, wantErr
				})
			}
			if _, err := GetJobPods(client, nil, dataselect.NoDataSelect, job.Namespace, job.Name); !errors.Is(err, wantErr) {
				t.Fatalf("GetJobPods() error = %v; want %v", err, wantErr)
			}
		})
	}
}
