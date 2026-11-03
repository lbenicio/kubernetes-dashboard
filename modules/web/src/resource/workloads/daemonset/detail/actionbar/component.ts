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

import {Component, DestroyRef, inject, OnInit, ChangeDetectionStrategy} from '@angular/core';
import {ActionbarService, ResourceMeta} from '@common/services/global/actionbar';
import {takeUntilDestroyed} from '@angular/core/rxjs-interop';
import {ActionbarDetailLogsComponent} from '../../../../../common/components/actionbar/detailactions/logs/component';
import {ActionbarDetailActionsComponent} from '../../../../../common/components/actionbar/detailactions/component';

@Component({
  selector: '',
  templateUrl: './template.html',
  changeDetection: ChangeDetectionStrategy.Eager,
  imports: [ActionbarDetailLogsComponent, ActionbarDetailActionsComponent],
})
export class ActionbarComponent implements OnInit {
  isInitialized = false;
  resourceMeta: ResourceMeta;

  private destroyRef = inject(DestroyRef);
  constructor(private readonly actionbar_: ActionbarService) {}

  ngOnInit(): void {
    this.actionbar_.onInit.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((resourceMeta: ResourceMeta) => {
      this.resourceMeta = resourceMeta;
      this.isInitialized = true;
    });
  }
}
