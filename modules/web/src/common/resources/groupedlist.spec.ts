// Copyright 2026 The Kubernetes Authors.
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

import {ListIdentifier, ListGroupIdentifier} from '../components/resourcelist/groupids';
import {GroupedResourceList} from './groupedlist';
import {emptyResourcesRatio} from '../components/workloadstatus/component';
import {OnListChangeEvent} from '@api/root.ui';
import {ResourceList} from '@api/root.api';

describe('GroupedResourceList', () => {
  beforeEach(() => {
    emptyResourcesRatio.cronJobRatio = [];
    emptyResourcesRatio.daemonSetRatio = [];
    emptyResourcesRatio.deploymentRatio = [];
    emptyResourcesRatio.jobRatio = [];
    emptyResourcesRatio.podRatio = [];
    emptyResourcesRatio.replicaSetRatio = [];
    emptyResourcesRatio.replicationControllerRatio = [];
    emptyResourcesRatio.statefulSetRatio = [];
  });

  const list = (status = {running: 1, failed: 1, pending: 1, succeeded: 1}, totalItems = 4): ResourceList =>
    ({status, listMeta: {totalItems}, cumulativeMetrics: []}) as never;

  it('tracks empty and filtered list zero states and group visibility', () => {
    const grouped = new GroupedResourceList();

    expect(grouped.shouldShowZeroState()).toBe(false);
    grouped.onListUpdate({
      id: ListIdentifier.pod,
      groupId: ListGroupIdentifier.workloads,
      items: 0,
      filtered: false,
      resourceList: list({running: 0, failed: 0, pending: 0, succeeded: 0}, 0),
    } as OnListChangeEvent);
    expect(grouped.shouldShowZeroState()).toBe(true);
    expect(grouped.isGroupVisible(ListGroupIdentifier.workloads)).toBe(false);

    grouped.onListUpdate({
      id: ListIdentifier.deployment,
      groupId: ListGroupIdentifier.workloads,
      items: 0,
      filtered: true,
      resourceList: list({running: 0, failed: 0, pending: 0, succeeded: 0}, 0),
    } as OnListChangeEvent);
    expect(grouped.shouldShowZeroState()).toBe(false);
    expect(grouped.isGroupVisible(ListGroupIdentifier.workloads)).toBe(false);
  });

  it('updates status ratios for supported workload lists', () => {
    const grouped = new GroupedResourceList();
    const ids = [
      ListIdentifier.cronJob,
      ListIdentifier.daemonSet,
      ListIdentifier.deployment,
      ListIdentifier.job,
      ListIdentifier.pod,
      ListIdentifier.replicaSet,
      ListIdentifier.replicationController,
      ListIdentifier.statefulSet,
    ];

    ids.forEach((id, index) => {
      grouped.onListUpdate({
        id,
        groupId: ListGroupIdentifier.workloads,
        items: 4,
        filtered: false,
        resourceList: list(),
      } as OnListChangeEvent);

      const ratio = [
        grouped.resourcesRatio.cronJobRatio,
        grouped.resourcesRatio.daemonSetRatio,
        grouped.resourcesRatio.deploymentRatio,
        grouped.resourcesRatio.jobRatio,
        grouped.resourcesRatio.podRatio,
        grouped.resourcesRatio.replicaSetRatio,
        grouped.resourcesRatio.replicationControllerRatio,
        grouped.resourcesRatio.statefulSetRatio,
      ][index];
      expect(ratio[0].value).toBe(25);
    });

    expect(grouped.resourcesRatio.cronJobRatio).toHaveLength(2);
    expect(grouped.resourcesRatio.daemonSetRatio).toHaveLength(3);
    expect(grouped.resourcesRatio.deploymentRatio).toHaveLength(3);
    expect(grouped.resourcesRatio.jobRatio).toHaveLength(4);
    expect(grouped.resourcesRatio.podRatio).toHaveLength(4);
    expect(grouped.cumulativeMetrics).toEqual([]);
    expect(grouped.resourcesRatio.replicaSetRatio).toHaveLength(3);
    expect(grouped.resourcesRatio.replicationControllerRatio).toHaveLength(3);
    expect(grouped.resourcesRatio.statefulSetRatio).toHaveLength(3);
  });

  it('leaves unrelated resource lists without workload ratios', () => {
    const grouped = new GroupedResourceList();
    grouped.onListUpdate({
      id: ListIdentifier.service,
      groupId: ListGroupIdentifier.discovery,
      items: 1,
      filtered: false,
      resourceList: list(),
    } as OnListChangeEvent);

    expect(grouped.resourcesRatio).toEqual({
      cronJobRatio: [],
      daemonSetRatio: [],
      deploymentRatio: [],
      jobRatio: [],
      podRatio: [],
      replicaSetRatio: [],
      replicationControllerRatio: [],
      statefulSetRatio: [],
    });
    expect(grouped.isGroupVisible(ListGroupIdentifier.discovery)).toBe(true);
  });
});
