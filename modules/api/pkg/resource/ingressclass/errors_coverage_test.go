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

package ingressclass

import (
	"errors"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func TestGetIngressClassListReturnsClientFailure(t *testing.T) {
	wantErr := errors.New("ingress class list failed")
	client := fake.NewSimpleClientset()
	client.PrependReactor("list", "ingressclasses", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, wantErr
	})
	if _, err := GetIngressClassList(client, dataselect.NoDataSelect); !errors.Is(err, wantErr) {
		t.Fatalf("GetIngressClassList() error = %v; want %v", err, wantErr)
	}
}
