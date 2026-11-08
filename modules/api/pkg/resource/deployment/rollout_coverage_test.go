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
	"errors"
	"testing"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func rolloutDeployment(revision string, paused bool) *apps.Deployment {
	return &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web", Namespace: "default", UID: types.UID("deployment-uid"),
			Annotations: map[string]string{RevisionAnnotationKey: revision},
		},
		Spec: apps.DeploymentSpec{
			Paused:   paused,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "web"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "web", Image: "web:v2"}}}},
		},
	}
}

func rolloutReplicaSet(revision, image string) *apps.ReplicaSet {
	controller := true
	return &apps.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web-" + revision, Namespace: "default", UID: types.UID("replicaset-" + revision),
			Labels:          map[string]string{"app": "web"},
			Annotations:     map[string]string{RevisionAnnotationKey: revision},
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: "web", UID: types.UID("deployment-uid"), Controller: &controller}},
		},
		Spec: apps.ReplicaSetSpec{Replicas: ptr(int32(1)), Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "web"}},
			Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "web", Image: image}}},
		}},
	}
}

func ptr(value int32) *int32 { return &value }

func TestPauseAndResumeDeployment(t *testing.T) {
	client := fake.NewSimpleClientset(rolloutDeployment("2", false))
	paused, err := PauseDeployment(client, "default", "web")
	if err != nil || paused == nil || !paused.Spec.Paused {
		t.Fatalf("PauseDeployment() = %#v, %v; want paused deployment", paused, err)
	}
	if _, err := PauseDeployment(client, "default", "web"); err == nil {
		t.Error("PauseDeployment(already paused) error = nil")
	}
	resumed, err := ResumeDeployment(client, "default", "web")
	if err != nil || resumed == nil || resumed.Spec.Paused {
		t.Fatalf("ResumeDeployment() = %#v, %v; want resumed deployment", resumed, err)
	}
	if _, err := ResumeDeployment(client, "default", "web"); err == nil {
		t.Error("ResumeDeployment(already resumed) error = nil")
	}
	client = fake.NewSimpleClientset(rolloutDeployment("2", true))
	if _, err := PauseDeployment(client, "default", "web"); err == nil {
		t.Error("PauseDeployment(already paused fixture) error = nil")
	}
}

func TestRestartDeploymentAddsRestartAnnotation(t *testing.T) {
	client := fake.NewSimpleClientset(rolloutDeployment("7", false))
	got, err := RestartDeployment(client, "default", "web")
	if err != nil || got == nil || got.Revision != "7" {
		t.Fatalf("RestartDeployment() = %#v, %v; want revision 7", got, err)
	}
	updated, err := client.AppsV1().Deployments("default").Get(t.Context(), "web", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Spec.Template.Annotations[RestartedAtAnnotationKey] == "" {
		t.Error("restart annotation was not added")
	}
}

func TestGetReplicaSetFromDeploymentFiltersByControllerOwner(t *testing.T) {
	deployment := rolloutDeployment("2", false)
	owned := rolloutReplicaSet("2", "web:v2")
	unowned := rolloutReplicaSet("3", "web:v3")
	unowned.OwnerReferences = nil
	client := fake.NewSimpleClientset(deployment, owned, unowned)
	got, err := GetReplicaSetFromDeployment(client, "default", "web")
	if err != nil || len(got) != 1 || got[0].Name != owned.Name {
		t.Fatalf("GetReplicaSetFromDeployment() = %#v, %v; want owned replica set only", got, err)
	}
}

func TestRollbackDeploymentSelectsRequestedRevision(t *testing.T) {
	deployment := rolloutDeployment("2", false)
	old := rolloutReplicaSet("1", "web:v1")
	current := rolloutReplicaSet("2", "web:v2")
	client := fake.NewSimpleClientset(deployment, old, current)

	got, err := RollbackDeployment(client, &RolloutSpec{Revision: "1"}, "default", "web")
	if err != nil || got == nil || got.Revision != "2" {
		t.Fatalf("RollbackDeployment() = %#v, %v; want current revision", got, err)
	}
	updated, err := client.AppsV1().Deployments("default").Get(t.Context(), "web", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Spec.Template.Spec.Containers[0].Image != "web:v1" {
		t.Errorf("rolled-back image = %q; want web:v1", updated.Spec.Template.Spec.Containers[0].Image)
	}
}

func TestRollbackDeploymentRejectsFirstRevisionAndUnknownRevision(t *testing.T) {
	client := fake.NewSimpleClientset(rolloutDeployment(FirstRevision, false))
	if _, err := RollbackDeployment(client, &RolloutSpec{Revision: "1"}, "default", "web"); err == nil {
		t.Error("RollbackDeployment(first revision) error = nil")
	}

	client = fake.NewSimpleClientset(rolloutDeployment("2", false), rolloutReplicaSet("2", "web:v2"))
	if _, err := RollbackDeployment(client, &RolloutSpec{Revision: "missing"}, "default", "web"); err == nil {
		t.Error("RollbackDeployment(unknown revision) error = nil")
	}
}

func TestDeploymentRolloutMethodsPropagateClientErrors(t *testing.T) {
	t.Run("get errors", func(t *testing.T) {
		client := fake.NewSimpleClientset()
		if _, err := PauseDeployment(client, "default", "missing"); err == nil {
			t.Error("PauseDeployment(missing) error = nil")
		}
		if _, err := ResumeDeployment(client, "default", "missing"); err == nil {
			t.Error("ResumeDeployment(missing) error = nil")
		}
		if _, err := RestartDeployment(client, "default", "missing"); err == nil {
			t.Error("RestartDeployment(missing) error = nil")
		}
		if _, err := RollbackDeployment(client, &RolloutSpec{Revision: "1"}, "default", "missing"); err == nil {
			t.Error("RollbackDeployment(missing) error = nil")
		}
		if _, err := GetReplicaSetFromDeployment(client, "default", "missing"); err == nil {
			t.Error("GetReplicaSetFromDeployment(missing) error = nil")
		}
	})

	t.Run("update errors", func(t *testing.T) {
		updateError := errors.New("update rejected")
		rejectUpdate := func(client *fake.Clientset) {
			client.PrependReactor("update", "deployments", func(clienttesting.Action) (bool, runtime.Object, error) {
				return true, nil, updateError
			})
		}

		client := fake.NewSimpleClientset(rolloutDeployment("2", false))
		rejectUpdate(client)
		if _, err := PauseDeployment(client, "default", "web"); !errors.Is(err, updateError) {
			t.Errorf("PauseDeployment() error = %v; want update error", err)
		}

		client = fake.NewSimpleClientset(rolloutDeployment("2", true))
		rejectUpdate(client)
		if _, err := ResumeDeployment(client, "default", "web"); !errors.Is(err, updateError) {
			t.Errorf("ResumeDeployment() error = %v; want update error", err)
		}

		client = fake.NewSimpleClientset(rolloutDeployment("2", false))
		rejectUpdate(client)
		if _, err := RestartDeployment(client, "default", "web"); !errors.Is(err, updateError) {
			t.Errorf("RestartDeployment() error = %v; want update error", err)
		}

		client = fake.NewSimpleClientset(rolloutDeployment("2", false), rolloutReplicaSet("1", "web:v1"))
		rejectUpdate(client)
		if _, err := RollbackDeployment(client, &RolloutSpec{Revision: "1"}, "default", "web"); !errors.Is(err, updateError) {
			t.Errorf("RollbackDeployment() error = %v; want update error", err)
		}
	})

	t.Run("replica set lookup errors", func(t *testing.T) {
		invalidSelector := rolloutDeployment("2", false)
		invalidSelector.Spec.Selector = &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "app", Operator: "INVALID"}}}
		if _, err := GetReplicaSetFromDeployment(fake.NewSimpleClientset(invalidSelector), "default", "web"); err == nil {
			t.Error("GetReplicaSetFromDeployment(invalid selector) error = nil")
		}

		listError := errors.New("replica set list rejected")
		client := fake.NewSimpleClientset(rolloutDeployment("2", false), rolloutReplicaSet("1", "web:v1"))
		client.PrependReactor("list", "replicasets", func(clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, listError
		})
		if _, err := GetReplicaSetFromDeployment(client, "default", "web"); !errors.Is(err, listError) {
			t.Errorf("GetReplicaSetFromDeployment() error = %v; want list error", err)
		}
		if _, err := RollbackDeployment(client, &RolloutSpec{Revision: "1"}, "default", "web"); !errors.Is(err, listError) {
			t.Errorf("RollbackDeployment() error = %v; want list error", err)
		}
	})
}
