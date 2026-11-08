// Copyright 2017 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"reflect"
	"testing"

	api "k8s.io/api/core/v1"
)

func getReplicasPointer(replicas int32) *int32 {
	return &replicas
}

func TestGetPodInfo(t *testing.T) {
	cases := []struct {
		current  int32
		desired  *int32
		pods     []api.Pod
		expected PodInfo
	}{
		{
			5,
			getReplicasPointer(4),
			[]api.Pod{
				{
					Status: api.PodStatus{
						Phase: api.PodRunning,
					},
				},
			},
			PodInfo{
				Current:  5,
				Desired:  getReplicasPointer(4),
				Running:  1,
				Pending:  0,
				Failed:   0,
				Warnings: make([]Event, 0),
			},
		},
	}

	for _, c := range cases {
		actual := GetPodInfo(c.current, c.desired, c.pods)
		if !reflect.DeepEqual(actual, c.expected) {
			t.Errorf("getPodInfo(%#v, %#v, %#v) == \n%#v\nexpected \n%#v\n",
				c.current, c.desired, c.pods, actual, c.expected)
		}
	}
}

func TestGetPodInfoCountsEachRecognizedPhase(t *testing.T) {
	info := GetPodInfo(5, nil, []api.Pod{
		{Status: api.PodStatus{Phase: api.PodRunning}},
		{Status: api.PodStatus{Phase: api.PodPending}},
		{Status: api.PodStatus{Phase: api.PodFailed}},
		{Status: api.PodStatus{Phase: api.PodSucceeded}},
		{Status: api.PodStatus{Phase: api.PodUnknown}},
		{Status: api.PodStatus{}},
	})
	if info.Current != 5 || info.Desired != nil || info.Running != 1 || info.Pending != 1 || info.Failed != 1 || info.Succeeded != 1 {
		t.Errorf("GetPodInfo() = %#v; want one pod in each recognized phase", info)
	}
	if info.Warnings == nil || len(info.Warnings) != 0 {
		t.Errorf("GetPodInfo().Warnings = %#v; want an initialized empty slice", info.Warnings)
	}
}
