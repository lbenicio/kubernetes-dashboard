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

package types

import (
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
)

func TestObjectMetaConversion(t *testing.T) {
	created := metav1.NewTime(time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC))
	input := metav1.ObjectMeta{
		Name:              "web",
		Namespace:         "default",
		Labels:            map[string]string{"app": "dashboard"},
		Annotations:       map[string]string{"owner": "platform"},
		CreationTimestamp: created,
		UID:               ktypes.UID("uid-1"),
		OwnerReferences: []metav1.OwnerReference{
			{Kind: "ReplicaSet", Name: "web-123"},
			{Kind: "Deployment", Name: "web"},
		},
	}

	got := NewObjectMeta(input)
	want := ObjectMeta{
		Name:              "web",
		Namespace:         "default",
		Labels:            input.Labels,
		Annotations:       input.Annotations,
		CreationTimestamp: created,
		UID:               ktypes.UID("uid-1"),
		OwnerReferences: []OwnerReference{
			{Kind: "ReplicaSet", Name: "web-123"},
			{Kind: "Deployment", Name: "web"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NewObjectMeta() = %#v, want %#v", got, want)
	}
	if refs := toOwnerReferences(nil); refs != nil {
		t.Fatalf("toOwnerReferences(nil) = %#v, want nil", refs)
	}
}

func TestResourceKindMetadata(t *testing.T) {
	for _, kind := range []ResourceKind{ResourceKindDeployment, ResourceKindReplicaSet, ResourceKindReplicationController, ResourceKindStatefulSet} {
		if !kind.Scalable() {
			t.Errorf("%q should be scalable", kind)
		}
	}
	for _, kind := range []ResourceKind{ResourceKindDeployment, ResourceKindDaemonSet, ResourceKindStatefulSet} {
		if !kind.Restartable() {
			t.Errorf("%q should be restartable", kind)
		}
	}
	if ResourceKind(ResourceKindPod).Scalable() || ResourceKind(ResourceKindPod).Restartable() {
		t.Fatal("pod should not be scalable or restartable")
	}
	if got := NewTypeMeta(ResourceKindDeployment); got != (TypeMeta{Kind: ResourceKindDeployment, Scalable: true, Restartable: true}) {
		t.Fatalf("NewTypeMeta(deployment) = %#v", got)
	}
	if ResourceKind(ResourceKindPod).String() != "pod" || Verb(VerbList).String() != "list" {
		t.Fatal("string conversion returned unexpected resource kind or verb")
	}
}
