// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package deployment

import (
	"testing"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	"k8s.io/dashboard/api/pkg/resource/common"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
)

func deploymentClientFixture() (*fake.Clientset, *apps.Deployment) {
	controlled := true
	replicas := int32(2)
	deployment := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", UID: types.UID("deployment-uid"), Labels: map[string]string{"app": "web"}},
		Spec: apps.DeploymentSpec{
			Replicas: replicasPtr(replicas),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "web"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "web", Image: "nginx:1"}}}},
		},
		Status: apps.DeploymentStatus{Replicas: replicas, ReadyReplicas: 1, UpdatedReplicas: 1},
	}
	newRS := &apps.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{Name: "web-new", Namespace: "default", UID: types.UID("new-rs-uid"), Labels: map[string]string{"app": "web"}, OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: deployment.Name, UID: deployment.UID, Controller: &controlled}}},
		Spec:       apps.ReplicaSetSpec{Replicas: replicasPtr(1), Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}}, Template: *deployment.Spec.Template.DeepCopy()},
		Status:     apps.ReplicaSetStatus{Replicas: 1, ReadyReplicas: 1},
	}
	oldTemplate := *deployment.Spec.Template.DeepCopy()
	oldTemplate.Spec.Containers[0].Image = "nginx:old"
	oldRS := &apps.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{Name: "web-old", Namespace: "default", UID: types.UID("old-rs-uid"), Labels: map[string]string{"app": "web"}, OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: deployment.Name, UID: deployment.UID, Controller: &controlled}}},
		Spec:       apps.ReplicaSetSpec{Replicas: replicasPtr(1), Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}}, Template: oldTemplate},
		Status:     apps.ReplicaSetStatus{Replicas: 1, ReadyReplicas: 1},
	}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web-0", Namespace: "default", UID: types.UID("pod-uid"), Labels: map[string]string{"app": "web"}, OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: newRS.Name, UID: newRS.UID, Controller: &controlled}}}, Status: corev1.PodStatus{Phase: corev1.PodRunning}}
	client := fake.NewSimpleClientset(deployment, newRS, oldRS, pod)
	return client, deployment
}

func replicasPtr(value int32) *int32 { return &value }

func TestDeploymentClientBackedListPodsAndReplicaSetViews(t *testing.T) {
	client, deployment := deploymentClientFixture()
	query := dataselect.NoDataSelect

	list, err := GetDeploymentList(client, common.NewSameNamespaceQuery("default"), query, nil)
	if err != nil || list.ListMeta.TotalItems != 1 || len(list.Deployments) != 1 || list.Deployments[0].ObjectMeta.Name != "web" {
		t.Fatalf("GetDeploymentList() = %#v, %v", list, err)
	}
	pods, err := GetDeploymentPods(client, nil, query, "default", deployment.Name)
	if err != nil || len(pods.Pods) != 1 || pods.Pods[0].ObjectMeta.Name != "web-0" {
		t.Fatalf("GetDeploymentPods() = %#v, %v", pods, err)
	}
	newReplicaSet, err := GetDeploymentNewReplicaSet(client, query, "default", deployment.Name)
	if err != nil || newReplicaSet.ObjectMeta.Name != "web-new" {
		t.Fatalf("GetDeploymentNewReplicaSet() = %#v, %v", newReplicaSet, err)
	}
	oldReplicaSets, err := GetDeploymentOldReplicaSets(client, query, "default", deployment.Name)
	if err != nil || oldReplicaSets.ListMeta.TotalItems != 1 || oldReplicaSets.ReplicaSets[0].ObjectMeta.Name != "web-old" {
		t.Fatalf("GetDeploymentOldReplicaSets() = %#v, %v", oldReplicaSets, err)
	}
	if _, err := GetDeploymentList(client, common.NewNamespaceQuery(nil), query, nil); err != nil {
		t.Fatalf("GetDeploymentList(all namespaces): %v", err)
	}
}

func TestDeployAppFromFileHandlesEmptyAndMalformedInput(t *testing.T) {
	deployed, err := DeployAppFromFile(&rest.Config{}, &AppDeploymentFromFileSpec{})
	if err != nil || !deployed {
		t.Fatalf("DeployAppFromFile(empty) = %v, %v; want EOF success", deployed, err)
	}
	deployed, err = DeployAppFromFile(&rest.Config{}, &AppDeploymentFromFileSpec{Content: "kind: [unclosed"})
	if err == nil || deployed {
		t.Fatalf("DeployAppFromFile(malformed) = %v, %v; want decode error", deployed, err)
	}
}
