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

package common

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
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

func TestResourceListChannelsFilterAllNamespacedWorkloads(t *testing.T) {
	client := clientfake.NewSimpleClientset(
		&networkingv1.Ingress{ObjectMeta: metaV1.ObjectMeta{Name: "keep", Namespace: "one"}},
		&networkingv1.Ingress{ObjectMeta: metaV1.ObjectMeta{Name: "drop", Namespace: "excluded"}},
		&corev1.Event{ObjectMeta: metaV1.ObjectMeta{Name: "keep", Namespace: "one"}},
		&corev1.Event{ObjectMeta: metaV1.ObjectMeta{Name: "drop", Namespace: "excluded"}},
		&corev1.ReplicationController{ObjectMeta: metaV1.ObjectMeta{Name: "keep", Namespace: "one"}},
		&corev1.ReplicationController{ObjectMeta: metaV1.ObjectMeta{Name: "drop", Namespace: "excluded"}},
		&appsv1.Deployment{ObjectMeta: metaV1.ObjectMeta{Name: "keep", Namespace: "one"}},
		&appsv1.Deployment{ObjectMeta: metaV1.ObjectMeta{Name: "drop", Namespace: "excluded"}},
		&appsv1.ReplicaSet{ObjectMeta: metaV1.ObjectMeta{Name: "keep", Namespace: "one"}},
		&appsv1.ReplicaSet{ObjectMeta: metaV1.ObjectMeta{Name: "drop", Namespace: "excluded"}},
		&appsv1.DaemonSet{ObjectMeta: metaV1.ObjectMeta{Name: "keep", Namespace: "one"}},
		&appsv1.DaemonSet{ObjectMeta: metaV1.ObjectMeta{Name: "drop", Namespace: "excluded"}},
		&batchv1.Job{ObjectMeta: metaV1.ObjectMeta{Name: "keep", Namespace: "one"}},
		&batchv1.Job{ObjectMeta: metaV1.ObjectMeta{Name: "drop", Namespace: "excluded"}},
		&batchv1.CronJob{ObjectMeta: metaV1.ObjectMeta{Name: "keep", Namespace: "one"}},
		&batchv1.CronJob{ObjectMeta: metaV1.ObjectMeta{Name: "drop", Namespace: "excluded"}},
		&appsv1.StatefulSet{ObjectMeta: metaV1.ObjectMeta{Name: "keep", Namespace: "one"}},
		&appsv1.StatefulSet{ObjectMeta: metaV1.ObjectMeta{Name: "drop", Namespace: "excluded"}},
		&corev1.ConfigMap{ObjectMeta: metaV1.ObjectMeta{Name: "keep", Namespace: "one"}},
		&corev1.ConfigMap{ObjectMeta: metaV1.ObjectMeta{Name: "drop", Namespace: "excluded"}},
		&corev1.Secret{ObjectMeta: metaV1.ObjectMeta{Name: "keep", Namespace: "one"}},
		&corev1.Secret{ObjectMeta: metaV1.ObjectMeta{Name: "drop", Namespace: "excluded"}},
	)
	namespaces := NewNamespaceQuery([]string{"one", "two"})

	ingresses := GetIngressListChannel(client, namespaces, 1)
	if got := <-ingresses.List; len(got.Items) != 1 || got.Items[0].Name != "keep" {
		t.Errorf("ingresses = %#v; want only the selected namespace's item", got.Items)
	}
	if err := <-ingresses.Error; err != nil {
		t.Fatalf("ingress list error = %v", err)
	}
	events := GetEventListChannel(client, namespaces, 1)
	if got := <-events.List; len(got.Items) != 1 || got.Items[0].Name != "keep" {
		t.Errorf("events = %#v; want only the selected namespace's item", got.Items)
	}
	if err := <-events.Error; err != nil {
		t.Fatalf("event list error = %v", err)
	}
	controllers := GetReplicationControllerListChannel(client, namespaces, 1)
	if got := <-controllers.List; len(got.Items) != 1 || got.Items[0].Name != "keep" {
		t.Errorf("replication controllers = %#v; want only the selected namespace's item", got.Items)
	}
	if err := <-controllers.Error; err != nil {
		t.Fatalf("replication controller list error = %v", err)
	}
	deployments := GetDeploymentListChannel(client, namespaces, 1)
	if got := <-deployments.List; len(got.Items) != 1 || got.Items[0].Name != "keep" {
		t.Errorf("deployments = %#v; want only the selected namespace's item", got.Items)
	}
	if err := <-deployments.Error; err != nil {
		t.Fatalf("deployment list error = %v", err)
	}
	replicaSets := GetReplicaSetListChannel(client, namespaces, 1)
	if got := <-replicaSets.List; len(got.Items) != 1 || got.Items[0].Name != "keep" {
		t.Errorf("replica sets = %#v; want only the selected namespace's item", got.Items)
	}
	if err := <-replicaSets.Error; err != nil {
		t.Fatalf("replica set list error = %v", err)
	}
	daemonSets := GetDaemonSetListChannel(client, namespaces, 1)
	if got := <-daemonSets.List; len(got.Items) != 1 || got.Items[0].Name != "keep" {
		t.Errorf("daemon sets = %#v; want only the selected namespace's item", got.Items)
	}
	if err := <-daemonSets.Error; err != nil {
		t.Fatalf("daemon set list error = %v", err)
	}
	jobs := GetJobListChannel(client, namespaces, 1)
	if got := <-jobs.List; len(got.Items) != 1 || got.Items[0].Name != "keep" {
		t.Errorf("jobs = %#v; want only the selected namespace's item", got.Items)
	}
	if err := <-jobs.Error; err != nil {
		t.Fatalf("job list error = %v", err)
	}
	cronJobs := GetCronJobListChannel(client, namespaces, 1)
	if got := <-cronJobs.List; len(got.Items) != 1 || got.Items[0].Name != "keep" {
		t.Errorf("cron jobs = %#v; want only the selected namespace's item", got.Items)
	}
	if err := <-cronJobs.Error; err != nil {
		t.Fatalf("cron job list error = %v", err)
	}
	statefulSets := GetStatefulSetListChannel(client, namespaces, 1)
	if got := <-statefulSets.List; len(got.Items) != 1 || got.Items[0].Name != "keep" {
		t.Errorf("stateful sets = %#v; want only the selected namespace's item", got.Items)
	}
	if err := <-statefulSets.Error; err != nil {
		t.Fatalf("stateful set list error = %v", err)
	}
	configMaps := GetConfigMapListChannel(client, namespaces, 1)
	if got := <-configMaps.List; len(got.Items) != 1 || got.Items[0].Name != "keep" {
		t.Errorf("config maps = %#v; want only the selected namespace's item", got.Items)
	}
	if err := <-configMaps.Error; err != nil {
		t.Fatalf("config map list error = %v", err)
	}
	secrets := GetSecretListChannel(client, namespaces, 1)
	if got := <-secrets.List; len(got.Items) != 1 || got.Items[0].Name != "keep" {
		t.Errorf("secrets = %#v; want only the selected namespace's item", got.Items)
	}
	if err := <-secrets.Error; err != nil {
		t.Fatalf("secret list error = %v", err)
	}
}
