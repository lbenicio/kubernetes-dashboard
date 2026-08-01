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

import {ChangeDetectorRef, Component, EventEmitter, Input, OnChanges, Output, SimpleChanges} from '@angular/core';
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
  standalone: false,
  selector: 'kd-workload-statuses',
  templateUrl: './template.html',
  styleUrls: ['./style.scss'],
})
export class WorkloadStatusComponent implements OnChanges {
  @Input() resourcesRatio = emptyResourcesRatio;
  @Output() filterByStatus = new EventEmitter<{resource: string; status: string}>();

  colors: string[] = [];
  animations = false;
  labels = true;
  trimLabels = false;
  size = [350, 250];
  chartReady = false;

  constructor(private cdr: ChangeDetectorRef) {}

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['resourcesRatio']) {
      const hasData = Object.values(this.resourcesRatio).some(arr => arr.length > 0);
      if (hasData && !this.chartReady) {
        this.chartReady = true;
        this.cdr.detectChanges();
        // Staggered resize events to catch flex-wrapped rows
        [100, 250, 500, 1000].forEach(d =>
          setTimeout(() => window.dispatchEvent(new Event('resize')), d),
        );
      }
    }
  }

  getCustomColor(label: string): string {
    if (label.includes($localize`Running: ${''}`)) return '#00c752';
    if (label.includes($localize`Succeeded: ${''}`)) return '#006028';
    if (label.includes($localize`Pending: ${''}`)) return '#ffad20';
    if (label.includes($localize`Failed: ${''}`)) return '#f00';
    return '';
  }

  onPieSelect(event: {name: string; status?: string}, resourceKey: string): void {
    if (!event) return;
    const resource = RESOURCE_ROUTES[resourceKey];
    if (!resource) return;
    const status = event.status || '';
    if (!status) return;
    this.filterByStatus.emit({resource, status});
  }
}
