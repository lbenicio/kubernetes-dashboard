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
import {ActivatedRoute, RouterLink} from '@angular/router';
import {CRDObject, CRDObjectList} from '@api/root.api';
import {Observable} from 'rxjs';
import {map} from 'rxjs/operators';
import {ResourceListBase} from '@common/resources/list';
import {NotificationsService} from '@common/services/global/notifications';
import {EndpointManager, Resource} from '@common/services/resource/endpoint';
import {NamespacedResourceService} from '@common/services/resource/resource';
import {MenuComponent} from '../../list/column/menu/component';
import {ListGroupIdentifier, ListIdentifier} from '../groupids';
import {takeUntilDestroyed} from '@angular/core/rxjs-interop';
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
import {MatSort} from '@angular/material/sort';
import {DateComponent} from '../../date/component';
import {ColumnComponent} from '../../list/column/component';
import {MatPaginator} from '@angular/material/paginator';
import {ListZeroStateComponent} from '../../list/zerostate/component';

@Component({
  selector: 'kd-crd-object-list',
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
    MatCellDef,
    MatCell,
    RouterLink,
    DateComponent,
    ColumnComponent,
    MatHeaderRowDef,
    MatHeaderRow,
    MatRowDef,
    MatRow,
    MatPaginator,
    ListZeroStateComponent,
  ],
})
export class CRDObjectListComponent extends ResourceListBase<CRDObjectList, CRDObject> {
  @Input() endpoint: string;
  @Input() namespaced = false;

  constructor(
    private readonly crdObject_: NamespacedResourceService<CRDObjectList>,
    private readonly activatedRoute_: ActivatedRoute,
    notifications: NotificationsService,
    cdr: ChangeDetectorRef
  ) {
    super(activatedRoute_.params.pipe(map(params => `customresourcedefinition/${params.crdName}`)), notifications, cdr);
    this.id = ListIdentifier.crdObject;
    this.groupId = ListGroupIdentifier.none;

    // Register action columns.
    this.registerActionColumn<MenuComponent>('menu', MenuComponent);

    this.activatedRoute_.params.pipe(takeUntilDestroyed(this.destroyRef)).subscribe(params => {
      this.endpoint = EndpointManager.resource(Resource.crd, true).child(params.crdName, Resource.crdObject);
    });
  }

  getResourceObservable(params?: HttpParams): Observable<CRDObjectList> {
    return this.crdObject_.get(this.endpoint, undefined, undefined, params);
  }

  map(crdObjectList: CRDObjectList): CRDObject[] {
    return crdObjectList.items;
  }

  getDisplayColumns(): string[] {
    return ['name', 'namespace', 'created'];
  }

  areMultipleNamespacesSelected(): boolean {
    return this.namespaceService_.areMultipleNamespacesSelected();
  }
}
