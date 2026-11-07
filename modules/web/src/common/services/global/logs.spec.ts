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

import {HttpParams} from '@angular/common/http';
import {LogService} from './logs';
import {of} from 'rxjs';

describe('LogService', () => {
  it('requests a resource log with optional query parameters', () => {
    const http = {get: jest.fn().mockReturnValue(of('log output'))};
    const service = new LogService(http as never);
    const params = new HttpParams().set('container', 'app');

    service.getResource('default/pod/web', params).subscribe();

    expect(http.get).toHaveBeenCalledWith('api/v1/log/default/pod/web', {params});
    expect(service.getResource('default/pod/web')).toBeTruthy();
  });

  it('tracks log display and refresh options', () => {
    const service = new LogService({} as never);

    expect(service.getFollowing()).toBe(false);
    expect(service.getAutoRefresh()).toBe(false);
    expect(service.getPrevious()).toBe(false);
    expect(service.getInverted()).toBe(false);
    expect(service.getCompact()).toBe(false);
    expect(service.getShowTimestamp()).toBe(false);

    service.setFollowing(true);
    service.toggleAutoRefresh();
    service.togglePrevious();
    service.toggleInverted();
    service.toggleCompact();
    service.toggleShowTimestamp();

    expect(service.getFollowing()).toBe(true);
    expect(service.getAutoRefresh()).toBe(true);
    expect(service.getPrevious()).toBe(true);
    expect(service.getInverted()).toBe(true);
    expect(service.getCompact()).toBe(true);
    expect(service.getShowTimestamp()).toBe(true);

    service.toggleAutoRefresh();
    service.togglePrevious();
    service.toggleInverted();
    service.toggleCompact();
    service.toggleShowTimestamp();
    expect(service.getAutoRefresh()).toBe(false);
    expect(service.getPrevious()).toBe(false);
    expect(service.getInverted()).toBe(false);
    expect(service.getCompact()).toBe(false);
    expect(service.getShowTimestamp()).toBe(false);
  });

  it('builds a stable log file name', () => {
    expect(new LogService({} as never).getLogFileName('web-0', 'nginx')).toBe('logs-from-nginx-in-web-0.log');
  });
});
