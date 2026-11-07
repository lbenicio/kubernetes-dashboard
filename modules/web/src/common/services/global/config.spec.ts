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

import {ConfigService} from './config';
import {of} from 'rxjs';

describe('ConfigService', () => {
  it('loads application configuration and tracks server time relative to initialization', () => {
    jest.useFakeTimers().setSystemTime(new Date('2026-01-01T00:00:00.000Z'));
    const config = {
      serverTime: Date.parse('2025-12-31T23:59:00.000Z'),
      userAgent: 'dashboard-test',
      version: '1.2.3',
    };
    const http = {get: jest.fn().mockReturnValue(of(config))};
    const service = new ConfigService(http as never);

    service.init();

    expect(http.get).toHaveBeenCalledWith('config');
    expect(service.getAppConfig()).toBeTruthy();
    expect(service.getUserAgent()).toBe('dashboard-test');
    expect(service.getVersion()).toBe('1.2.3');
    expect(service.getServerTime()).toEqual(new Date(config.serverTime));

    jest.advanceTimersByTime(2_000);
    expect(service.getServerTime()).toEqual(new Date(config.serverTime + 2_000));
    jest.useRealTimers();
  });

  it('uses the client clock when the server did not provide a timestamp', () => {
    jest.useFakeTimers().setSystemTime(new Date('2026-01-01T00:00:00.000Z'));
    const service = new ConfigService({get: jest.fn().mockReturnValue(of({}))} as never);
    service.init();

    expect(service.getServerTime()).toEqual(new Date('2026-01-01T00:00:00.000Z'));
    jest.useRealTimers();
  });
});
