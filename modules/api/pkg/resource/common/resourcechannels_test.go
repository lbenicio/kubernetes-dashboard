// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

package common

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	apiextensionsfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientfake "k8s.io/client-go/kubernetes/fake"
)

func assertListChannel[T any](t *testing.T, list <-chan *T, errs <-chan error, reads int) {
	t.Helper()
	for i := 0; i < reads; i++ {
		if got := <-list; got == nil {
			t.Fatalf("list read %d is nil", i)
		}
		if err := <-errs; err != nil {
			t.Fatalf("error read %d = %v; want nil", i, err)
		}
	}
}

func TestResourceListChannelsBroadcastListsAndErrors(t *testing.T) {
	client := clientfake.NewSimpleClientset()
	extensionsClient := apiextensionsfake.NewSimpleClientset()
	namespaces := NewNamespaceQuery([]string{"one", "two"})
	const reads = 2

	service := GetServiceListChannel(client, namespaces, reads)
	assertListChannel(t, service.List, service.Error, reads)
	ingress := GetIngressListChannel(client, namespaces, reads)
	assertListChannel(t, ingress.List, ingress.Error, reads)
	limitRanges := GetLimitRangeListChannel(client, namespaces, reads)
	assertListChannel(t, limitRanges.List, limitRanges.Error, reads)
	nodes := GetNodeListChannel(client, reads)
	assertListChannel(t, nodes.List, nodes.Error, reads)
	namespaceList := GetNamespaceListChannel(client, reads)
	assertListChannel(t, namespaceList.List, namespaceList.Error, reads)
	events := GetEventListChannel(client, namespaces, reads)
	assertListChannel(t, events.List, events.Error, reads)
	eventsWithOptions := GetEventListChannelWithOptions(client, namespaces, metaV1.ListOptions{Limit: 5}, reads)
	assertListChannel(t, eventsWithOptions.List, eventsWithOptions.Error, reads)
	endpointSlices := GetEndpointSliceListChannel(client, namespaces, reads)
	assertListChannel(t, endpointSlices.List, endpointSlices.Error, reads)
	endpointSlicesWithOptions := GetEndpointSliceListChannelWithOptions(client, namespaces, metaV1.ListOptions{Limit: 5}, reads)
	assertListChannel(t, endpointSlicesWithOptions.List, endpointSlicesWithOptions.Error, reads)
	pods := GetPodListChannel(client, namespaces, reads)
	assertListChannel(t, pods.List, pods.Error, reads)
	podsWithOptions := GetPodListChannelWithOptions(client, namespaces, metaV1.ListOptions{Limit: 5}, reads)
	assertListChannel(t, podsWithOptions.List, podsWithOptions.Error, reads)
	replicationControllers := GetReplicationControllerListChannel(client, namespaces, reads)
	assertListChannel(t, replicationControllers.List, replicationControllers.Error, reads)
	deployments := GetDeploymentListChannel(client, namespaces, reads)
	assertListChannel(t, deployments.List, deployments.Error, reads)
	replicaSets := GetReplicaSetListChannel(client, namespaces, reads)
	assertListChannel(t, replicaSets.List, replicaSets.Error, reads)
	replicaSetsWithOptions := GetReplicaSetListChannelWithOptions(client, namespaces, metaV1.ListOptions{Limit: 5}, reads)
	assertListChannel(t, replicaSetsWithOptions.List, replicaSetsWithOptions.Error, reads)
	daemonSets := GetDaemonSetListChannel(client, namespaces, reads)
	assertListChannel(t, daemonSets.List, daemonSets.Error, reads)
	jobs := GetJobListChannel(client, namespaces, reads)
	assertListChannel(t, jobs.List, jobs.Error, reads)
	cronJobs := GetCronJobListChannel(client, namespaces, reads)
	assertListChannel(t, cronJobs.List, cronJobs.Error, reads)
	statefulSets := GetStatefulSetListChannel(client, namespaces, reads)
	assertListChannel(t, statefulSets.List, statefulSets.Error, reads)
	configMaps := GetConfigMapListChannel(client, namespaces, reads)
	assertListChannel(t, configMaps.List, configMaps.Error, reads)
	secrets := GetSecretListChannel(client, namespaces, reads)
	assertListChannel(t, secrets.List, secrets.Error, reads)
	roles := GetRoleListChannel(client, namespaces, reads)
	assertListChannel(t, roles.List, roles.Error, reads)
	clusterRoles := GetClusterRoleListChannel(client, reads)
	assertListChannel(t, clusterRoles.List, clusterRoles.Error, reads)
	roleBindings := GetRoleBindingListChannel(client, namespaces, reads)
	assertListChannel(t, roleBindings.List, roleBindings.Error, reads)
	clusterRoleBindings := GetClusterRoleBindingListChannel(client, reads)
	assertListChannel(t, clusterRoleBindings.List, clusterRoleBindings.Error, reads)
	persistentVolumes := GetPersistentVolumeListChannel(client, reads)
	assertListChannel(t, persistentVolumes.List, persistentVolumes.Error, reads)
	persistentVolumeClaims := GetPersistentVolumeClaimListChannel(client, namespaces, reads)
	assertListChannel(t, persistentVolumeClaims.List, persistentVolumeClaims.Error, reads)
	podDisruptionBudgets := GetPodDisruptionBudgetListChannel(client, namespaces, reads)
	assertListChannel(t, podDisruptionBudgets.List, podDisruptionBudgets.Error, reads)
	customResourceDefinitions := GetCustomResourceDefinitionChannelV1(extensionsClient, reads)
	assertListChannel(t, customResourceDefinitions.List, customResourceDefinitions.Error, reads)
	resourceQuotas := GetResourceQuotaListChannel(client, namespaces, reads)
	assertListChannel(t, resourceQuotas.List, resourceQuotas.Error, reads)
	horizontalPodAutoscalers := GetHorizontalPodAutoscalerListChannel(client, namespaces, reads)
	assertListChannel(t, horizontalPodAutoscalers.List, horizontalPodAutoscalers.Error, reads)
	storageClasses := GetStorageClassListChannel(client, reads)
	assertListChannel(t, storageClasses.List, storageClasses.Error, reads)
	ingressClasses := GetIngressClassListChannel(client, reads)
	assertListChannel(t, ingressClasses.List, ingressClasses.Error, reads)
}

func TestResourceListChannelsFilterNamespacedResults(t *testing.T) {
	client := clientfake.NewSimpleClientset(
		&corev1.Service{ObjectMeta: metaV1.ObjectMeta{Name: "keep", Namespace: "one"}},
		&corev1.Service{ObjectMeta: metaV1.ObjectMeta{Name: "drop", Namespace: "excluded"}},
		&corev1.Pod{ObjectMeta: metaV1.ObjectMeta{Name: "keep", Namespace: "one"}},
		&corev1.Pod{ObjectMeta: metaV1.ObjectMeta{Name: "drop", Namespace: "excluded"}},
	)
	namespaces := NewNamespaceQuery([]string{"one", "two"})

	serviceChannel := GetServiceListChannel(client, namespaces, 1)
	if got := <-serviceChannel.List; len(got.Items) != 1 || got.Items[0].Name != "keep" {
		t.Errorf("services = %#v; want only the selected namespace's item", got.Items)
	}
	if err := <-serviceChannel.Error; err != nil {
		t.Fatalf("service list error = %v", err)
	}

	podChannel := GetPodListChannel(client, namespaces, 1)
	if got := <-podChannel.List; len(got.Items) != 1 || got.Items[0].Name != "keep" {
		t.Errorf("pods = %#v; want only the selected namespace's item", got.Items)
	}
	if err := <-podChannel.Error; err != nil {
		t.Fatalf("pod list error = %v", err)
	}
}
