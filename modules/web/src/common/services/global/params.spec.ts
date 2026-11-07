/*
 * Copyright 2017 The Kubernetes Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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

import {NavigationEnd} from '@angular/router';
import {BehaviorSubject, Subject} from 'rxjs';
import {ParamsService} from './params';

describe('ParamsService', () => {
  it('copies active route and query parameters after navigation', () => {
    const events = new Subject<unknown>();
    const activeRoute = {
      firstChild: null as never,
      params: new BehaviorSubject({namespace: 'default'}),
      queryParams: new BehaviorSubject({filterBy: 'status,Running'}),
    };
    const service = new ParamsService({events} as never, {firstChild: activeRoute} as never);
    const onChange = jest.fn();
    service.onParamChange.subscribe(onChange);

    events.next({type: 'unrelated'});
    expect(onChange).not.toHaveBeenCalled();

    events.next(new NavigationEnd(1, '/pods', '/pods'));

    expect(service.getQueryParam('filterBy')).toBe('status,Running');
    expect(onChange).toHaveBeenCalledTimes(1);
    service.setQueryParam('page', '2');
    expect(service.getQueryParam('page')).toBe('2');
  });

  it('uses the route itself when it has no active child and keeps absent keys undefined', () => {
    const events = new Subject<unknown>();
    const route = {
      firstChild: null as never,
      params: new BehaviorSubject({name: 'web'}),
      queryParams: new BehaviorSubject({}),
    };
    const service = new ParamsService({events} as never, route as never);

    events.next(new NavigationEnd(1, '/pod/web', '/pod/web'));

    expect(service.getQueryParam('missing')).toBeUndefined();
    expect(service.getQueryParam('name')).toBeUndefined();
  });
});
