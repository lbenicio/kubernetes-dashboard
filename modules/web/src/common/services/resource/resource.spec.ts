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

import {HttpClient, HttpParams} from '@angular/common/http';
import {fakeAsync, tick} from '@angular/core/testing';
import {ReplaySubject, of} from 'rxjs';
import {NamespacedResourceService, ResourceService} from './resource';

describe('ResourceService', () => {
  const params = new HttpParams().set('page', 2);

  it('substitutes the resource name and passes request parameters', fakeAsync(() => {
    const settingsUpdate = new ReplaySubject<void>(1);
    const settings = {
      onSettingsUpdate: settingsUpdate,
      getResourceAutoRefreshTimeInterval: jest.fn().mockReturnValue(0),
    };
    const http = {get: jest.fn().mockReturnValue(of({items: ['pod']}))};
    const result = jest.fn();
    const service = new ResourceService(http as unknown as HttpClient, settings as never);

    settingsUpdate.next();
    const subscription = service.get('api/v1/pod/:name', 'web', params).subscribe(result);
    tick();

    expect(http.get).toHaveBeenCalledWith('api/v1/pod/web', {params});
    expect(result).toHaveBeenCalledWith({items: ['pod']});
    subscription.unsubscribe();
  }));

  it('refreshes when settings change and honors the configured interval', fakeAsync(() => {
    const settingsUpdate = new ReplaySubject<void>(1);
    const settings = {
      onSettingsUpdate: settingsUpdate,
      getResourceAutoRefreshTimeInterval: jest.fn().mockReturnValue(2),
    };
    const http = {get: jest.fn().mockReturnValue(of({}))};
    const result = jest.fn();
    const service = new ResourceService(http as unknown as HttpClient, settings as never);

    settingsUpdate.next();
    const subscription = service.get('api/v1/pod').subscribe(result);
    tick(0);
    expect(http.get).toHaveBeenCalledTimes(1);

    tick(2000);
    expect(result).toHaveBeenCalledTimes(2);

    settingsUpdate.next();
    tick(0);
    expect(result).toHaveBeenCalledTimes(3);
    subscription.unsubscribe();
  }));
});

describe('NamespacedResourceService', () => {
  it('uses the selected namespace and supports an explicit namespace override', fakeAsync(() => {
    const settingsUpdate = new ReplaySubject<void>(1);
    const settings = {
      onSettingsUpdate: settingsUpdate,
      getResourceAutoRefreshTimeInterval: jest.fn().mockReturnValue(0),
    };
    const namespace = {
      current: jest.fn().mockReturnValue('default'),
      isMultiNamespace: jest.fn().mockReturnValue(false),
    };
    const http = {get: jest.fn().mockReturnValue(of({}))};
    const service = new NamespacedResourceService(http as unknown as HttpClient, namespace as never, settings as never);
    settingsUpdate.next();

    const currentSubscription = service.get('api/v1/pod/:namespace/:name', 'web').subscribe();
    tick();
    expect(http.get).toHaveBeenLastCalledWith('api/v1/pod/default/web', {params: undefined});
    currentSubscription.unsubscribe();

    const overrideSubscription = service.get('api/v1/pod/:namespace/:name', 'web', 'other').subscribe();
    tick();
    expect(http.get).toHaveBeenLastCalledWith('api/v1/pod/other/web', {params: undefined});
    expect(namespace.current).toHaveBeenCalledTimes(1);
    overrideSubscription.unsubscribe();
  }));

  it('uses a space for the all-namespaces API route', fakeAsync(() => {
    const settingsUpdate = new ReplaySubject<void>(1);
    const settings = {
      onSettingsUpdate: settingsUpdate,
      getResourceAutoRefreshTimeInterval: jest.fn().mockReturnValue(0),
    };
    const namespace = {
      current: jest.fn().mockReturnValue('_all'),
      isMultiNamespace: jest.fn().mockReturnValue(true),
    };
    const http = {get: jest.fn().mockReturnValue(of({}))};
    const service = new NamespacedResourceService(http as unknown as HttpClient, namespace as never, settings as never);
    settingsUpdate.next();

    const subscription = service.get('api/v1/pod/:namespace').subscribe();
    tick();

    expect(http.get).toHaveBeenCalledWith('api/v1/pod/ ', {params: undefined});
    subscription.unsubscribe();
  }));
});
