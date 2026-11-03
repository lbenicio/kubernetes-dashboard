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

import {Component, ChangeDetectionStrategy} from '@angular/core';

import {ListGroupIdentifier} from '@common/components/resourcelist/groupids';
import {GroupedResourceList} from '@common/resources/groupedlist';
import {WorkloadStatusComponent} from '../../common/components/workloadstatus/component';
import {CronJobListComponent} from '../../common/components/resourcelist/cronjob/component';
import {DaemonSetListComponent} from '../../common/components/resourcelist/daemonset/component';
import {DeploymentListComponent} from '../../common/components/resourcelist/deployment/component';
import {JobListComponent} from '../../common/components/resourcelist/job/component';
import {PodListComponent} from '../../common/components/resourcelist/pod/component';
import {ReplicaSetListComponent} from '../../common/components/resourcelist/replicaset/component';
import {ReplicationControllerListComponent} from '../../common/components/resourcelist/replicationcontroller/component';
import {StatefulSetListComponent} from '../../common/components/resourcelist/statefulset/component';
import {ZeroStateComponent} from '../../common/components/zerostate/component';

@Component({
  selector: 'kd-workloads',
  templateUrl: './template.html',
  changeDetection: ChangeDetectionStrategy.Eager,
  imports: [
    WorkloadStatusComponent,
    CronJobListComponent,
    DaemonSetListComponent,
    DeploymentListComponent,
    JobListComponent,
    PodListComponent,
    ReplicaSetListComponent,
    ReplicationControllerListComponent,
    StatefulSetListComponent,
    ZeroStateComponent,
  ],
})
export class WorkloadsComponent extends GroupedResourceList {
  hasWorkloads(): boolean {
    return this.isGroupVisible(ListGroupIdentifier.workloads);
  }

  hasDiscovery(): boolean {
    return this.isGroupVisible(ListGroupIdentifier.discovery);
  }

  hasConfig(): boolean {
    return this.isGroupVisible(ListGroupIdentifier.config);
  }

  showWorkloadStatuses(): boolean {
    return Object.values(this.resourcesRatio).reduce((sum, ratioItems) => sum + ratioItems.length, 0) !== 0;
  }
}
