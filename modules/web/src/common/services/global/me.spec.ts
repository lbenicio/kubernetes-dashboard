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

import {MeService} from './me';
import {EventEmitter} from '@angular/core';
import {EMPTY, of, Subject, throwError} from 'rxjs';

describe('MeService', () => {
  it('loads the current user without caching the response', () => {
    const http = {get: jest.fn().mockReturnValue(of({authenticated: true, name: 'Ada'}))};
    const service = new MeService(http as never);

    service.me().subscribe();

    expect(http.get).toHaveBeenCalledWith('api/v1/me', expect.any(Object));
    expect(http.get.mock.calls[0][1].headers.get('Cache-Control')).toBe('no-cache');
  });

  it('initializes the user and handles interval refreshes', async () => {
    const initialUser = {authenticated: true, name: 'Ada'};
    const refreshedUser = {authenticated: true, name: 'Grace'};
    const initialResponse = new Subject<typeof initialUser>();
    const interval = new Subject<number>();
    const http = {
      get: jest.fn().mockReturnValueOnce(initialResponse).mockReturnValue(of(refreshedUser)),
    };
    const service = new MeService(http as never);
    (service as unknown as {_interval: unknown})._interval = interval;
    (service as unknown as {_initialized: EventEmitter<void>})._initialized = new EventEmitter<void>(false);

    const initialized = service.init();
    initialResponse.next(initialUser);
    await initialized;

    expect(service.getUser()).toBe(initialUser);
    expect(service.getUserName()).toBe('Ada');

    interval.next(0);

    expect(service.getUser()).toBe(refreshedUser);
    expect(http.get).toHaveBeenCalledTimes(2);
  });

  it('uses an unauthenticated user when initialization fails', async () => {
    const initialResponse = new Subject<{authenticated: boolean}>();
    const http = {get: jest.fn().mockReturnValue(initialResponse)};
    const service = new MeService(http as never);
    (service as unknown as {_interval: unknown})._interval = EMPTY;
    (service as unknown as {_initialized: EventEmitter<void>})._initialized = new EventEmitter<void>(false);

    const initialized = service.init();
    initialResponse.error(new Error('unavailable'));
    await initialized;

    expect(service.getUser()).toEqual({authenticated: false});
  });

  it('refreshes and stores a user, resets it, and propagates refresh errors', async () => {
    const user = {authenticated: true, name: 'Lin'};
    const http = {get: jest.fn().mockReturnValue(of(user))};
    const service = new MeService(http as never);

    await expect(service.refresh()).resolves.toBe(user);
    expect(service.getUser()).toBe(user);

    service.reset();
    expect(service.getUser()).toEqual({});
    expect(service.getUserName()).toBeUndefined();

    http.get.mockReturnValue(throwError(() => new Error('unavailable')));
    await expect(service.refresh()).rejects.toThrow('unavailable');
  });
});
