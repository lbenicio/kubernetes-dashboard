// Copyright 2017 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import {Component, EventEmitter, Input, Output} from '@angular/core';
import {ResourcesRatio} from '@api/root.ui';

export const emptyResourcesRatio: ResourcesRatio = {
  cronJobRatio: [],
  daemonSetRatio: [],
  deploymentRatio: [],
  jobRatio: [],
  podRatio: [],
  replicaSetRatio: [],
  replicationControllerRatio: [],
  statefulSetRatio: [],
};

/** Maps a resource ratio key to its resource list route. */
const RESOURCE_ROUTES: Record<string, string> = {
  cronJobRatio: 'cronjob',
  daemonSetRatio: 'daemonset',
  deploymentRatio: 'deployment',
  jobRatio: 'job',
  podRatio: 'pod',
  replicaSetRatio: 'replicaset',
  replicationControllerRatio: 'replicationcontroller',
  statefulSetRatio: 'statefulset',
};

@Component({
  selector: 'kd-workload-statuses',
  templateUrl: './template.html',
  styleUrls: ['./style.scss'],
})
export class WorkloadStatusComponent {
  @Input() resourcesRatio = emptyResourcesRatio;
  @Output() filterByStatus = new EventEmitter<{resource: string; status: string}>();

  colors: string[] = [];
  animations = false;
  labels = true;
  trimLabels = false;
  size = [350, 250];

  getCustomColor(label: string): string {
    if (label.includes($localize`Running: ${''}`)) {
      return '#00c752';
    } else if (label.includes($localize`Succeeded: ${''}`)) {
      return '#006028';
    } else if (label.includes($localize`Pending: ${''}`)) {
      return '#ffad20';
    } else if (label.includes($localize`Failed: ${''}`)) {
      return '#f00';
    }
    return '';
  }

  /**
   * Handles clicks on pizza chart slices.
   * Emits a filter event to show only resources with the selected status.
   */
  onPieSelect(event: {name: string; status?: string}, resourceKey: string): void {
    if (!event) return;

    const resource = RESOURCE_ROUTES[resourceKey];
    if (!resource) return;

    // Use the status field from the RatioItem if available
    const status = event.status || '';
    if (!status) return;

    this.filterByStatus.emit({resource, status});
  }
}
