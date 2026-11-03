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
import {ClusterRoleBindingListComponent} from '../../common/components/resourcelist/clusterrolebinding/component';
import {ClusterRoleListComponent} from '../../common/components/resourcelist/clusterrole/component';
import {NamespaceListComponent} from '../../common/components/resourcelist/namespace/component';
import {NetworkPolicyListComponent} from '../../common/components/resourcelist/networkpolicy/component';
import {NodeListComponent} from '../../common/components/resourcelist/node/component';
import {PersistentVolumeListComponent} from '../../common/components/resourcelist/persistentvolume/component';
import {RoleBindingListComponent} from '../../common/components/resourcelist/rolebinding/component';
import {RoleListComponent} from '../../common/components/resourcelist/role/component';
import {ServiceAccountListComponent} from '../../common/components/resourcelist/serviceaccount/component';
import {ZeroStateComponent} from '../../common/components/zerostate/component';

@Component({
  selector: 'kd-cluster',
  templateUrl: './template.html',
  changeDetection: ChangeDetectionStrategy.Eager,
  imports: [
    ClusterRoleBindingListComponent,
    ClusterRoleListComponent,
    NamespaceListComponent,
    NetworkPolicyListComponent,
    NodeListComponent,
    PersistentVolumeListComponent,
    RoleBindingListComponent,
    RoleListComponent,
    ServiceAccountListComponent,
    ZeroStateComponent,
  ],
})
export class ClusterComponent extends GroupedResourceList {}
