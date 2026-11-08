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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"

	"k8s.io/dashboard/api/pkg/resource/common"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestJobListAndEventsReturnKubernetesFailures(t *testing.T) {
	for _, resource := range []string{"jobs", "pods", "events"} {
		t.Run("list "+resource, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			client.PrependReactor("list", resource, func(ktesting.Action) (bool, runtime.Object, error) {
				return true, nil, errors.New("simulated Kubernetes failure")
			})
			if _, err := GetJobList(client, common.NewNamespaceQuery([]string{"default"}), dataselect.NoDataSelect, nil); err == nil {
				t.Fatalf("GetJobList() returned no error for failed %s list", resource)
			}
		})
	}

	client := fake.NewSimpleClientset()
	client.PrependReactor("list", "events", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("event list failed")
	})
	if _, err := GetJobEvents(client, dataselect.NoDataSelect, "default", "backup"); err == nil {
		t.Fatal("GetJobEvents() returned no error after event listing failed")
	}
}
