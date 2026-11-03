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
import {GroupedResourceList} from '@common/resources/groupedlist';
import {ConfigMapListComponent} from '../../common/components/resourcelist/configmap/component';
import {PersistentVolumeClaimListComponent} from '../../common/components/resourcelist/persistentvolumeclaim/component';
import {SecretListComponent} from '../../common/components/resourcelist/secret/component';
import {StorageClassListComponent} from '../../common/components/resourcelist/storageclass/component';
import {ZeroStateComponent} from '../../common/components/zerostate/component';

@Component({
  selector: 'kd-config',
  templateUrl: './template.html',
  changeDetection: ChangeDetectionStrategy.Eager,
  imports: [
    ConfigMapListComponent,
    PersistentVolumeClaimListComponent,
    SecretListComponent,
    StorageClassListComponent,
    ZeroStateComponent,
  ],
})
export class ConfigComponent extends GroupedResourceList {}
