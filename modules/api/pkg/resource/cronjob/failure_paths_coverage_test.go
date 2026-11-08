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

package cronjob

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

func TestCronJobHelpersHandleRelatedAPIFailures(t *testing.T) {
	if got := getStatus(nil); got.Running != 0 || got.Failed != 0 {
		t.Fatalf("getStatus(nil) = %#v, want empty status", got)
	}

	cronJob := &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: "default", UID: "cron-uid"}}
	for _, test := range []struct {
		name     string
		resource string
		wantErr  bool
	}{
		{name: "job list error is ignored", resource: "jobs"},
		{name: "pod list error", resource: "pods", wantErr: true},
		{name: "event list error", resource: "events", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := fake.NewSimpleClientset(cronJob)
			client.PrependReactor("list", test.resource, func(ktesting.Action) (bool, runtime.Object, error) {
				return true, nil, errors.New("simulated Kubernetes failure")
			})
			_, err := GetCronJobJobs(client, nil, dataselect.NoDataSelect, "default", "backup", true)
			if (err != nil) != test.wantErr {
				t.Fatalf("GetCronJobJobs() error = %v; wantErr=%t", err, test.wantErr)
			}
		})
	}

	client := fake.NewSimpleClientset(cronJob)
	client.PrependReactor("list", "events", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("event list failed")
	})
	if _, err := GetCronJobEvents(client, dataselect.NoDataSelect, "default", "backup"); err == nil {
		t.Fatal("GetCronJobEvents() returned no error after event listing failed")
	}

	client = fake.NewSimpleClientset(cronJob)
	client.PrependReactor("create", "jobs", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("job creation failed")
	})
	if err := TriggerCronJob(client, "default", "backup"); err == nil {
		t.Fatal("TriggerCronJob() returned no error after job creation failed")
	}

}
