// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cronjob

import (
	"testing"
	"time"

	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	"k8s.io/dashboard/api/pkg/resource/common"
	"k8s.io/dashboard/api/pkg/resource/dataselect"
	dashboardtypes "k8s.io/dashboard/types"
)

func TestCronJobCellAndConversion(t *testing.T) {
	created := metav1.NewTime(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	cronJob := batch.CronJob{ObjectMeta: metav1.ObjectMeta{Name: "cleanup", Namespace: "default", UID: types.UID("uid"), CreationTimestamp: created}}
	cell := CronJobCell(cronJob)
	for _, tc := range []struct {
		property dataselect.PropertyName
		want     dataselect.ComparableValue
	}{
		{dataselect.NameProperty, dataselect.StdComparableString("cleanup")},
		{dataselect.NamespaceProperty, dataselect.StdComparableString("default")},
		{dataselect.CreationTimestampProperty, dataselect.StdComparableTime(created.Time)},
	} {
		if got := cell.GetProperty(tc.property); got != tc.want {
			t.Errorf("GetProperty(%s) = %v; want %v", tc.property, got, tc.want)
		}
	}
	if cell.GetProperty(dataselect.PropertyName("unsupported")) != nil {
		t.Error("unsupported property should return nil")
	}
	selector := cell.GetResourceSelector()
	if selector.Namespace != "default" || selector.ResourceType != dashboardtypes.ResourceKindCronJob || selector.ResourceName != "cleanup" || selector.UID != "uid" {
		t.Errorf("GetResourceSelector() = %#v", selector)
	}
	if got := FromCells(ToCells([]batch.CronJob{cronJob})); len(got) != 1 || got[0].Name != cronJob.Name {
		t.Errorf("cell conversion = %#v", got)
	}
}

func TestCronJobListAndOwnedJobQueries(t *testing.T) {
	suspend := false
	cronJob := &batch.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: "cleanup", Namespace: "default", UID: types.UID("cron-uid")},
		Spec:       batch.CronJobSpec{Schedule: "*/5 * * * *", Suspend: &suspend, JobTemplate: batch.JobTemplateSpec{Spec: batch.JobSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "cleanup", Image: "busybox:1"}}}}}}},
	}
	owner := metav1.OwnerReference{Kind: "CronJob", Name: cronJob.Name, UID: cronJob.UID}
	client := fake.NewSimpleClientset(
		cronJob,
		&batch.Job{ObjectMeta: metav1.ObjectMeta{Name: "active-owned", Namespace: "default", OwnerReferences: []metav1.OwnerReference{owner}}, Status: batch.JobStatus{Active: 1}},
		&batch.Job{ObjectMeta: metav1.ObjectMeta{Name: "complete-owned", Namespace: "default", OwnerReferences: []metav1.OwnerReference{owner}}, Status: batch.JobStatus{Succeeded: 1}},
		&batch.Job{ObjectMeta: metav1.ObjectMeta{Name: "unrelated", Namespace: "default", OwnerReferences: []metav1.OwnerReference{{UID: "other"}}}, Status: batch.JobStatus{Active: 1}},
	)

	list, err := GetCronJobList(client, common.NewSameNamespaceQuery("default"), dataselect.NoDataSelect)
	if err != nil || list.ListMeta.TotalItems != 1 || list.Items[0].Schedule != "*/5 * * * *" || list.Items[0].ContainerImages[0] != "busybox:1" || list.Status.Running != 1 {
		t.Fatalf("GetCronJobList() = %#v, %v", list, err)
	}
	active, err := GetCronJobJobs(client, nil, dataselect.NoDataSelect, "default", "cleanup", true)
	if err != nil || active.ListMeta.TotalItems != 1 || active.Jobs[0].ObjectMeta.Name != "active-owned" {
		t.Fatalf("GetCronJobJobs(active) = %#v, %v", active, err)
	}
	complete, err := GetCronJobJobs(client, nil, dataselect.NoDataSelect, "default", "cleanup", false)
	if err != nil || complete.ListMeta.TotalItems != 1 || complete.Jobs[0].ObjectMeta.Name != "complete-owned" {
		t.Fatalf("GetCronJobJobs(completed) = %#v, %v", complete, err)
	}
	if _, err := GetCronJobJobs(client, nil, dataselect.NoDataSelect, "default", "missing", true); err == nil {
		t.Fatal("GetCronJobJobs() succeeded for missing cron job")
	}
}

func TestCronJobJobFilters(t *testing.T) {
	jobs := []batch.Job{
		{ObjectMeta: metav1.ObjectMeta{Name: "owned", OwnerReferences: []metav1.OwnerReference{{UID: "cron"}}}, Status: batch.JobStatus{Active: 1}},
		{ObjectMeta: metav1.ObjectMeta{Name: "unowned"}, Status: batch.JobStatus{Succeeded: 1}},
		{ObjectMeta: metav1.ObjectMeta{Name: "other-owner", OwnerReferences: []metav1.OwnerReference{{UID: "other"}}}, Status: batch.JobStatus{Succeeded: 1}},
	}
	owned := filterJobsByOwnerUID("cron", jobs)
	if len(owned) != 1 || owned[0].Name != "owned" {
		t.Fatalf("filterJobsByOwnerUID() = %#v", owned)
	}
	if got := filterJobsByState(true, jobs); len(got) != 1 || got[0].Name != "owned" {
		t.Fatalf("filterJobsByState(active) = %#v", got)
	}
	if got := filterJobsByState(false, jobs); len(got) != 2 {
		t.Fatalf("filterJobsByState(completed) = %#v", got)
	}
}
