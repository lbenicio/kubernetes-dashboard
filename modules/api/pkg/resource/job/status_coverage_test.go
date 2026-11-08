// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package job

import (
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/dashboard/api/pkg/resource/common"
)

func TestGetStatusAndJobStatusClassifyLifecycle(t *testing.T) {
	if got := getStatus(nil, nil); got != (common.ResourceStatus{}) {
		t.Fatalf("getStatus(nil) = %#v; want empty status", got)
	}

	jobs := []batchv1.Job{
		statusTestJob("failed", batchv1.JobFailed),
		statusTestJob("complete", batchv1.JobComplete),
		statusTestJob("running", ""),
		statusTestJob("pending", ""),
	}
	jobs[2].Status.Active = 1
	jobs[3].Status.Active = 1
	pods := []corev1.Pod{{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Labels: map[string]string{"job": "running"}},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}}

	got := getStatus(&batchv1.JobList{Items: jobs}, pods)
	want := common.ResourceStatus{Failed: 1, Succeeded: 1, Running: 1, Pending: 1}
	if got != want {
		t.Fatalf("getStatus() = %#v; want %#v", got, want)
	}
	if status := getJobStatus(&jobs[0]); status.Status != JobStatusFailed || status.Message != string(batchv1.JobFailed) {
		t.Fatalf("failed job status = %#v", status)
	}
	if status := getJobStatus(&jobs[1]); status.Status != JobStatusComplete {
		t.Fatalf("complete job status = %#v", status)
	}
	if status := getJobStatus(&jobs[2]); status.Status != JobStatusRunning {
		t.Fatalf("running job status = %#v", status)
	}
}

func statusTestJob(name string, conditionType batchv1.JobConditionType) batchv1.Job {
	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: batchv1.JobSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"job": name}},
		},
	}
	if conditionType != "" {
		job.Status.Conditions = []batchv1.JobCondition{{Type: conditionType, Status: corev1.ConditionTrue, Message: string(conditionType)}}
	}
	return job
}
