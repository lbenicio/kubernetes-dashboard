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

import {HttpParams} from '@angular/common/http';
import {ChangeDetectionStrategy, ChangeDetectorRef, Component, Input} from '@angular/core';
import {HorizontalPodAutoscaler, HorizontalPodAutoscalerList} from '@api/root.api';
import {Observable} from 'rxjs';
import {ResourceListWithStatuses} from '@common/resources/list';
import {NotificationsService} from '@common/services/global/notifications';
import {EndpointManager, Resource} from '@common/services/resource/endpoint';
import {NamespacedResourceService} from '@common/services/resource/resource';
import {MenuComponent} from '../../list/column/menu/component';
import {ListGroupIdentifier, ListIdentifier} from '../groupids';
import {CardComponent} from '../../card/component';
import {CardListFilterComponent} from '../../list/filter/component';
import {LoadingSpinnerComponent} from '../../list/spinner/component';
import {
  MatTable,
  MatColumnDef,
  MatHeaderCellDef,
  MatHeaderCell,
  MatCellDef,
  MatCell,
  MatHeaderRowDef,
  MatHeaderRow,
  MatRowDef,
  MatRow,
} from '@angular/material/table';
import {MatSort, MatSortHeader} from '@angular/material/sort';
import {DateComponent} from '../../date/component';
import {ColumnComponent} from '../../list/column/component';
import {NgClass} from '@angular/common';
import {MatPaginator} from '@angular/material/paginator';
import {ListZeroStateComponent} from '../../list/zerostate/component';

@Component({
  selector: 'kd-horizontal-pod-autoscaler-list',
  templateUrl: './template.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [
    CardComponent,
    CardListFilterComponent,
    LoadingSpinnerComponent,
    MatTable,
    MatSort,
    MatColumnDef,
    MatHeaderCellDef,
    MatHeaderCell,
    MatSortHeader,
    MatCellDef,
    MatCell,
    DateComponent,
    ColumnComponent,
    MatHeaderRowDef,
    MatHeaderRow,
    MatRowDef,
    MatRow,
    NgClass,
    MatPaginator,
    ListZeroStateComponent,
  ],
})
export class HorizontalPodAutoscalerListComponent extends ResourceListWithStatuses<
  HorizontalPodAutoscalerList,
  HorizontalPodAutoscaler
> {
  @Input() endpoint = EndpointManager.resource(Resource.horizontalPodAutoscaler, true).list();

  constructor(
    private readonly horizontalpodautoscaler_: NamespacedResourceService<HorizontalPodAutoscalerList>,
    notifications: NotificationsService,
    cdr: ChangeDetectorRef
  ) {
    super('horizontalpodautoscaler', notifications, cdr);
    this.id = ListIdentifier.horizontalpodautoscaler;
    this.groupId = ListGroupIdentifier.workloads;

    // Register action columns.
    this.registerActionColumn<MenuComponent>('menu', MenuComponent);

    // Register dynamic columns.
    this.registerDynamicColumn('namespace', 'name', this.shouldShowNamespaceColumn_.bind(this));
  }

  getResourceObservable(params?: HttpParams): Observable<HorizontalPodAutoscalerList> {
    return this.horizontalpodautoscaler_.get(this.endpoint, undefined, undefined, params);
  }

  map(horizontalPodAutoscalerList: HorizontalPodAutoscalerList): HorizontalPodAutoscaler[] {
    return horizontalPodAutoscalerList.horizontalpodautoscalers;
  }

  getDisplayColumns(): string[] {
    return ['name', 'minpods', 'maxpods', 'scaletargetref', 'created'];
  }

  private shouldShowNamespaceColumn_(): boolean {
    return this.namespaceService_.areMultipleNamespacesSelected();
  }
}
