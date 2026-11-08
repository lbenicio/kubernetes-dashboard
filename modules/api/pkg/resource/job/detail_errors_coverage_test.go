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

	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestGetJobDetailReturnsKubernetesFailures(t *testing.T) {
	wantErr := errors.New("job request failed")
	client := fake.NewSimpleClientset()
	client.PrependReactor("get", "jobs", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, wantErr
	})
	if _, err := GetJobDetail(client, "default", "task"); !errors.Is(err, wantErr) {
		t.Fatalf("GetJobDetail() error = %v, want %v", err, wantErr)
	}

	job := &batch.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "task", Namespace: "default"},
		Spec:       batch.JobSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"job": "task"}}},
	}
	client = fake.NewSimpleClientset(job)
	client.PrependReactor("list", "pods", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, &corev1.PodList{}, wantErr
	})
	if _, err := GetJobDetail(client, "default", "task"); !errors.Is(err, wantErr) {
		t.Fatalf("GetJobDetail() pod list error = %v, want %v", err, wantErr)
	}
}
