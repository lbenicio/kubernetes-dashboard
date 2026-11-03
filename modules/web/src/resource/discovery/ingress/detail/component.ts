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

import {Component, DestroyRef, inject, OnDestroy, OnInit, ChangeDetectionStrategy} from '@angular/core';
import {ActivatedRoute} from '@angular/router';
import {IngressDetail} from '@api/root.api';

import {ActionbarService, ResourceMeta} from '@common/services/global/actionbar';
import {NotificationsService} from '@common/services/global/notifications';
import {EndpointManager, Resource} from '@common/services/resource/endpoint';
import {NamespacedResourceService} from '@common/services/resource/resource';
import {takeUntilDestroyed} from '@angular/core/rxjs-interop';
import {ObjectMetaComponent} from '../../../../common/components/objectmeta/component';
import {CardComponent} from '../../../../common/components/card/component';
import {PropertyComponent} from '../../../../common/components/property/component';
import {ExternalEndpointComponent} from '../../../../common/components/endpoint/external/component';
import {IngressRuleFlatListComponent} from '../../../../common/components/ingressrulelist/component';
import {EventListComponent} from '../../../../common/components/resourcelist/event/component';

@Component({
  selector: 'kd-ingress-detail',
  templateUrl: './template.html',
  changeDetection: ChangeDetectionStrategy.Eager,
  imports: [
    ObjectMetaComponent,
    CardComponent,
    PropertyComponent,
    ExternalEndpointComponent,
    IngressRuleFlatListComponent,
    EventListComponent,
  ],
})
export class IngressDetailComponent implements OnInit, OnDestroy {
  ingress: IngressDetail;
  isInitialized = false;
  eventListEndpoint: string;
  private readonly endpoint_ = EndpointManager.resource(Resource.ingress, true);

  private destroyRef = inject(DestroyRef);
  constructor(
    private readonly ingress_: NamespacedResourceService<IngressDetail>,
    private readonly actionbar_: ActionbarService,
    private readonly activatedRoute_: ActivatedRoute,
    private readonly notifications_: NotificationsService
  ) {}

  ngOnInit(): void {
    const resourceName = this.activatedRoute_.snapshot.params.resourceName;
    const resourceNamespace = this.activatedRoute_.snapshot.params.resourceNamespace;

    this.eventListEndpoint = this.endpoint_.child(resourceName, Resource.event, resourceNamespace);

    this.ingress_
      .get(this.endpoint_.detail(), resourceName, resourceNamespace)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe((d: IngressDetail) => {
        this.ingress = d;
        this.notifications_.pushErrors(d.errors);
        this.actionbar_.onInit.emit(new ResourceMeta('Ingress', d.objectMeta, d.typeMeta));
        this.isInitialized = true;
      });
  }

  ngOnDestroy(): void {
    this.actionbar_.onDetailsLeave.emit();
  }
}
