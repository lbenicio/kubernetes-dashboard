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
	"testing"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFindAndBuildReplicaSetTemplates(t *testing.T) {
	deployment := &apps.Deployment{Spec: apps.DeploymentSpec{Template: corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "web"}},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "web", Image: "web:v2"}}},
	}}}
	newReplicaSet := &apps.ReplicaSet{ObjectMeta: metav1.ObjectMeta{UID: "new"}, Spec: apps.ReplicaSetSpec{Template: deployment.Spec.Template}}
	oldReplicaSet := &apps.ReplicaSet{ObjectMeta: metav1.ObjectMeta{UID: "old"}, Spec: apps.ReplicaSetSpec{Template: corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "web"}},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "web", Image: "web:v1"}}},
	}}}

	if got := GetNewReplicaSetTemplate(deployment); got.Spec.Containers[0].Image != "web:v2" {
		t.Errorf("GetNewReplicaSetTemplate() = %#v", got)
	}
	if got := FindNewReplicaSet(deployment, []*apps.ReplicaSet{oldReplicaSet, newReplicaSet}); got != newReplicaSet {
		t.Errorf("FindNewReplicaSet() = %v; want new replica set", got)
	}
	if got := FindNewReplicaSet(deployment, []*apps.ReplicaSet{oldReplicaSet}); got != nil {
		t.Errorf("FindNewReplicaSet(no match) = %v; want nil", got)
	}

	zeroReplicas := int32(0)
	oneReplica := int32(1)
	oldReplicaSet.Spec.Replicas = &oneReplica
	emptyReplicaSet := &apps.ReplicaSet{ObjectMeta: metav1.ObjectMeta{UID: "empty"}, Spec: apps.ReplicaSetSpec{Replicas: &zeroReplicas}}
	required, all, err := FindOldReplicaSets(deployment, []*apps.ReplicaSet{newReplicaSet, oldReplicaSet, emptyReplicaSet})
	if err != nil || len(required) != 1 || required[0] != oldReplicaSet || len(all) != 2 {
		t.Fatalf("FindOldReplicaSets() = %v, %v, %v; want one required and two total", required, all, err)
	}
	if _, all, err := FindOldReplicaSets(deployment, []*apps.ReplicaSet{oldReplicaSet, emptyReplicaSet}); err != nil || len(all) != 2 {
		t.Errorf("FindOldReplicaSets(no new set) = %v, %v", all, err)
	}
}
