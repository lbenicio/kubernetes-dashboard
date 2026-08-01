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

import {Component, AfterViewInit} from '@angular/core';

import {ListGroupIdentifier} from '@common/components/resourcelist/groupids';
import {GroupedResourceList} from '@common/resources/groupedlist';

@Component({
  standalone: false,
  selector: 'kd-overview',
  templateUrl: './template.html',
})
export class OverviewComponent extends GroupedResourceList implements AfterViewInit {
  /** The currently active status filter for all resource lists. */
  statusFilter = '';

  hasCluster(): boolean {
    return this.isGroupVisible(ListGroupIdentifier.cluster);
  }

  hasWorkloads(): boolean {
    return this.isGroupVisible(ListGroupIdentifier.workloads);
  }

  hasDiscovery(): boolean {
    return this.isGroupVisible(ListGroupIdentifier.discovery);
  }

  hasConfig(): boolean {
    return this.isGroupVisible(ListGroupIdentifier.config);
  }

  ngAfterViewInit(): void {
    // ngx-charts pie charts may not render on initial load because
    // flex layout hasn't resolved container dimensions. We dispatch
    // resize events at multiple intervals to ensure charts pick up
    // their container sizes once layout has settled.
    [100, 300, 600].forEach(delay => {
      setTimeout(() => window.dispatchEvent(new Event('resize')), delay);
    });
  }

  showWorkloadStatuses(): boolean {
    return Object.values(this.resourcesRatio).reduce((sum, ratioItems) => sum + ratioItems.length, 0) !== 0;
  }

  /**
   * Handles clicks on pizza chart slices.
   * Filters all resource lists to show only resources with the given status.
   * Clicking the same status again clears the filter.
   */
  onStatusFilterChange(event: {resource: string; status: string}): void {
    if (this.statusFilter === event.status) {
      // Toggle off
      this.statusFilter = '';
    } else {
      this.statusFilter = event.status;
    }
  }
}
