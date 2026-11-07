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

import {NavigationEnd, NavigationStart} from '@angular/router';
import {KdStateService} from './state';
import {Subject} from 'rxjs';

describe('KdStateService', () => {
  it('emits before and after navigation events', () => {
    const events = new Subject<unknown>();
    const service = new KdStateService({events} as never);
    const before = jest.fn();
    const success = jest.fn();
    service.onBefore.subscribe(before);
    service.onSuccess.subscribe(success);

    events.next(new NavigationStart(1, '/pods'));
    events.next(new NavigationEnd(1, '/pods', '/pods'));
    events.next({type: 'unrelated'});

    expect(before).toHaveBeenCalledTimes(1);
    expect(success).toHaveBeenCalledTimes(1);
  });

  it('builds resource paths from optional parts', () => {
    const service = new KdStateService({events: new Subject()} as never);

    expect(service.href('overview')).toBe('/overview');
    expect(service.href('pod', 'api', 'default', 'container')).toBe('/pod/default/api/container');
    expect(service.href('namespace', '', 'default')).toBe('/namespace/default');
  });
});
