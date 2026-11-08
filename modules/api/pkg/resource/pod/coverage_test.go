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

package pod

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	integrationapi "k8s.io/dashboard/api/pkg/integration/api"
	metricapi "k8s.io/dashboard/api/pkg/integration/metric/api"
	"k8s.io/dashboard/api/pkg/resource/common"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
	dashboardtypes "k8s.io/dashboard/types"
)

type podCoverageMetricClient struct{}

func (podCoverageMetricClient) DownloadMetric(selectors []metricapi.ResourceSelector, name string, _ *metricapi.CachedResources) metricapi.MetricPromises {
	promises := metricapi.NewMetricPromises(1)
	metric := metricapi.Metric{MetricName: name, Label: metricapi.Label{dashboardtypes.ResourceKindPod: {apimachinerytypes.UID("pod-uid")}}, MetricPoints: []metricapi.MetricPoint{{Value: 10}, {Value: 20}}}
	promises.PutMetrics([]metricapi.Metric{metric}, nil)
	return promises
}
func (podCoverageMetricClient) DownloadMetrics([]metricapi.ResourceSelector, []string, *metricapi.CachedResources) metricapi.MetricPromises {
	return nil
}
func (podCoverageMetricClient) AggregateMetrics(promises metricapi.MetricPromises, _ string, _ metricapi.AggregationModes) metricapi.MetricPromises {
	return promises
}
func (podCoverageMetricClient) ID() integrationapi.IntegrationID { return "pod-coverage" }
func (podCoverageMetricClient) HealthCheck() error               { return nil }

func TestPodCellPropertiesAndSelector(t *testing.T) {
	created := metav1.NewTime(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	pod := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", UID: apimachinerytypes.UID("uid"), CreationTimestamp: created}, Status: corev1.PodStatus{Phase: corev1.PodRunning}}
	cell := PodCell(pod)
	for _, tc := range []struct {
		property dataselect.PropertyName
		want     dataselect.ComparableValue
	}{
		{dataselect.NameProperty, dataselect.StdComparableString("web")},
		{dataselect.NamespaceProperty, dataselect.StdComparableString("default")},
		{dataselect.CreationTimestampProperty, dataselect.StdComparableTime(created.Time)},
		{dataselect.StatusProperty, dataselect.StdComparableString("Running")},
	} {
		if got := cell.GetProperty(tc.property); got != tc.want {
			t.Errorf("GetProperty(%s) = %v; want %v", tc.property, got, tc.want)
		}
	}
	if cell.GetProperty(dataselect.PropertyName("unsupported")) != nil {
		t.Error("unsupported property should return nil")
	}
	selector := cell.GetResourceSelector()
	if selector.Namespace != "default" || selector.ResourceType != dashboardtypes.ResourceKindPod || selector.ResourceName != "web" || selector.UID != "uid" {
		t.Errorf("GetResourceSelector() = %#v", selector)
	}
	if got := fromCells(toCells([]corev1.Pod{pod})); len(got) != 1 || got[0].Name != pod.Name {
		t.Errorf("cell conversion = %#v", got)
	}
}

func TestPodConditionsAndAttachedMetrics(t *testing.T) {
	probe := metav1.NewTime(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	transition := metav1.NewTime(probe.Add(-time.Minute))
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "web", UID: "pod-uid"},
		Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{
			Type: corev1.PodReady, Status: corev1.ConditionTrue,
			LastProbeTime: probe, LastTransitionTime: transition,
			Reason: "ContainersReady", Message: "pod is ready",
		}}},
	}
	conditions := getPodConditions(pod)
	if len(conditions) != 1 || conditions[0].Type != string(corev1.PodReady) || conditions[0].Status != corev1.ConditionTrue ||
		!conditions[0].LastProbeTime.Equal(&probe) || !conditions[0].LastTransitionTime.Equal(&transition) ||
		conditions[0].Reason != "ContainersReady" || conditions[0].Message != "pod is ready" {
		t.Errorf("getPodConditions() = %#v", conditions)
	}

	cpu := uint64(250)
	metrics := &MetricsByPod{MetricsMap: map[apimachinerytypes.UID]PodMetrics{
		"pod-uid": {CPUUsage: &cpu, CPUUsageHistory: []metricapi.MetricPoint{{Value: cpu}}},
	}}
	view := toPod(&pod, metrics, nil)
	if view.Metrics == nil || view.Metrics.CPUUsage == nil || *view.Metrics.CPUUsage != cpu || len(view.Metrics.CPUUsageHistory) != 1 {
		t.Errorf("toPod() metrics = %#v; want metrics for pod UID", view.Metrics)
	}
}

func TestPodResourceRequestsLimitsAndGPUAllocations(t *testing.T) {
	pod := &corev1.Pod{Spec: corev1.PodSpec{
		Containers: []corev1.Container{{Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m"), corev1.ResourceMemory: resource.MustParse("64Mi"), corev1.ResourceName("nvidia.com/gpu"): resource.MustParse("1")},
			Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("128Mi"), corev1.ResourceName("amd.com/gpu"): resource.MustParse("1")},
		}}},
		InitContainers: []corev1.Container{{Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("32Mi")},
			Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2"), corev1.ResourceMemory: resource.MustParse("256Mi")},
		}}},
		Overhead: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("10Mi")},
	}}
	resources, err := getPodAllocatedResources(pod)
	if err != nil {
		t.Fatalf("getPodAllocatedResources() error = %v", err)
	}
	if resources.CPURequests != 1100 || resources.CPULimits != 2100 || resources.MemoryRequests != 74*1024*1024 || resources.MemoryLimits != 266*1024*1024 {
		t.Errorf("allocated resources = %#v", resources)
	}
	if len(resources.GPURequests) != 1 || resources.GPURequests[0].Type != NvidiaGPU || resources.GPURequests[0].Quantity != 1 || len(resources.GPULimits) != 1 || resources.GPULimits[0].Type != AMDGPU {
		t.Errorf("GPU allocation = requests %#v, limits %#v", resources.GPURequests, resources.GPULimits)
	}
	if got := ToGPU("vendor.example/gpu-card"); got != UnknownGPU {
		t.Errorf("ToGPU(unknown gpu) = %q", got)
	}
}

func TestPodResourceListAggregation(t *testing.T) {
	resources := corev1.ResourceList{
		corev1.ResourceCPU: resource.MustParse("2"),
	}
	maxResourceList(resources, corev1.ResourceList{
		corev1.ResourceCPU:                    resource.MustParse("1"),
		corev1.ResourceMemory:                 resource.MustParse("64Mi"),
		corev1.ResourceName("nvidia.com/gpu"): resource.MustParse("1"),
	})
	if got := resources[corev1.ResourceCPU]; got.Cmp(resource.MustParse("2")) != 0 {
		t.Errorf("maxResourceList() lowered CPU from 2 to %s", got.String())
	}
	if got := resources[corev1.ResourceMemory]; got.Cmp(resource.MustParse("64Mi")) != 0 {
		t.Errorf("maxResourceList() memory = %s; want 64Mi", got.String())
	}

	allocations := toGPUAllocations(corev1.ResourceList{
		corev1.ResourceName("gpu.intel.com/xe"):        resource.MustParse("1"),
		corev1.ResourceName("gpu.intel.com/iris"):      resource.MustParse("2"),
		corev1.ResourceName("vendor.example/gpu"):      resource.MustParse("3"),
		corev1.ResourceName("example.com/accelerator"): resource.MustParse("4"),
	})
	if len(allocations) != 2 {
		t.Fatalf("toGPUAllocations() returned %#v; want merged Intel and unknown GPU allocations", allocations)
	}
	gotByType := make(map[GPU]int64, len(allocations))
	for _, allocation := range allocations {
		gotByType[allocation.Type] = allocation.Quantity
	}
	if gotByType[IntelGPU] != 3 || gotByType[UnknownGPU] != 3 {
		t.Errorf("GPU allocation quantities = %#v; want intel=3 and unknown=3", gotByType)
	}
}

func TestPodInitializationTerminalStateAndVolumeHelpers(t *testing.T) {
	restartAlways := corev1.ContainerRestartPolicyAlways
	status := corev1.PodStatus{Conditions: []corev1.PodCondition{{Type: corev1.PodInitialized, Status: corev1.ConditionTrue}, {Type: corev1.PodReady, Status: corev1.ConditionTrue}}}
	if !isPodInitializedConditionTrue(&status) || !hasPodReadyCondition(status.Conditions) || !isRestartableInitContainer(&corev1.Container{RestartPolicy: &restartAlways}) {
		t.Fatal("pod initialization helpers rejected true conditions")
	}
	if isPodInitializedConditionTrue(&corev1.PodStatus{}) || hasPodReadyCondition(nil) || isRestartableInitContainer(nil) || isRestartableInitContainer(&corev1.Container{}) {
		t.Fatal("pod initialization helpers accepted absent conditions")
	}
	if !IsPodPhaseTerminal(corev1.PodFailed) || !IsPodPhaseTerminal(corev1.PodSucceeded) || IsPodPhaseTerminal(corev1.PodRunning) {
		t.Fatal("IsPodPhaseTerminal() returned unexpected values")
	}
	pod := &corev1.Pod{Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: "config", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}}}}
	container := corev1.Container{Name: "web", VolumeMounts: []corev1.VolumeMount{{Name: "config", MountPath: "/etc/config"}, {Name: "missing", MountPath: "/missing"}}}
	mounts := extractContainerMounts(container, pod)
	if len(mounts) != 2 || mounts[0].Volume.Name != "config" || mounts[1].Volume.Name != "" || getVolume(pod.Spec.Volumes, "absent").Name != "" {
		t.Fatalf("volume helpers returned %#v", mounts)
	}
	status.ContainerStatuses = []corev1.ContainerStatus{{Name: "web", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}
	containerStatus, state := extractContainerStatus(&corev1.Pod{Status: status}, &container)
	if containerStatus == nil || state.Running == nil {
		t.Fatalf("extractContainerStatus() = %#v, %#v", containerStatus, state)
	}
	if _, state := extractContainerStatus(&corev1.Pod{}, &container); state.Running != nil {
		t.Fatalf("missing container status returned %#v", state)
	}
}

func TestPodMetricUIDAndPerPodMetrics(t *testing.T) {
	if _, err := getPodUIDFromMetric(metricapi.Metric{}); err == nil {
		t.Fatal("getPodUIDFromMetric() accepted missing labels")
	}
	if _, err := getPodUIDFromMetric(metricapi.Metric{Label: metricapi.Label{dashboardtypes.ResourceKindPod: {"a", "b"}}}); err == nil {
		t.Fatal("getPodUIDFromMetric() accepted multiple UIDs")
	}
	uid, err := getPodUIDFromMetric(metricapi.Metric{Label: metricapi.Label{dashboardtypes.ResourceKindPod: {"pod-uid"}}})
	if err != nil || uid != "pod-uid" {
		t.Fatalf("getPodUIDFromMetric() = %q, %v", uid, err)
	}
	query := dataselect.NewDataSelectQuery(dataselect.NoPagination, dataselect.NoSort, dataselect.NoFilter, dataselect.NewMetricQuery([]string{metricapi.CpuUsage, metricapi.MemoryUsage}, nil))
	metrics, err := getMetricsPerPod([]corev1.Pod{{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", UID: "pod-uid"}}}, podCoverageMetricClient{}, query)
	if err != nil || len(metrics.MetricsMap) != 1 {
		t.Fatalf("getMetricsPerPod() = %#v, %v", metrics, err)
	}
	podMetrics := metrics.MetricsMap["pod-uid"]
	if podMetrics.CPUUsage == nil || *podMetrics.CPUUsage != 20 || podMetrics.MemoryUsage == nil || *podMetrics.MemoryUsage != 20 || len(podMetrics.CPUUsageHistory) != 2 || len(podMetrics.MemoryUsageHistory) != 2 {
		t.Errorf("pod metrics = %#v", podMetrics)
	}
}

func TestGetPodListClient(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"}, Status: corev1.PodStatus{Phase: corev1.PodPending}})
	list, err := GetPodList(client, nil, common.NewSameNamespaceQuery("default"), dataselect.NoDataSelect)
	if err != nil || list.ListMeta.TotalItems != 1 || len(list.Pods) != 1 || list.Pods[0].ObjectMeta.Name != "web" {
		t.Fatalf("GetPodList() = %#v, %v", list, err)
	}
}
