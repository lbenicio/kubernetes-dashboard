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
import {NodeListComponent as NodeListComponent_1} from '../../../../common/components/resourcelist/node/component';

@Component({
  selector: 'kd-node-list-state',
  template: '<kd-node-list [showMetrics]="true"></kd-node-list>',
  changeDetection: ChangeDetectionStrategy.Eager,
  imports: [NodeListComponent_1],
})
export class NodeListComponent {}
